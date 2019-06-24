package maildav

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	"github.com/tarent/logrus"
	"github.com/targodan/go-errors"
)

type Poller struct {
	config *PollerConfig
}

type DestinationInfo struct {
	Config    *DestinationConfig
	Directory string
}

type Attachment struct {
	Filename        string
	Content         []byte
	DestinationInfo *DestinationInfo
}

func NewPoller(config *PollerConfig) (*Poller, error) {
	return &Poller{
		config: config,
	}, nil
}

func (p *Poller) StartPolling(ctx context.Context, uploader *Uploader) error {
	run := true
	for run {
		attachments, err := p.Poll()
		if err != nil {
			logrus.WithField("source", p.config.SourceName).WithError(err).Error("Error while polling.")
		}

		err = uploader.UploadAttachments(attachments)
		if err != nil {
			logrus.WithField("source", p.config.SourceName).WithError(err).Error("Error while uploading.")
		}

		logrus.WithField("source", p.config.SourceName).WithError(ctx.Err()).Infof("Going to sleep for %v.", p.config.Timeout)
		select {
		case <-ctx.Done():
			logrus.WithField("source", p.config.SourceName).WithError(ctx.Err()).Info("Context canceled.")
			run = false
		case <-time.After(p.config.Timeout):
			logrus.WithField("source", p.config.SourceName).WithError(ctx.Err()).Info("Woke up.")
		}
	}
	return nil
}

func (p *Poller) Poll() ([]*Attachment, error) {
	attachments := []*Attachment{}

	logrus.WithField("source", p.config.SourceName).Info("Connecting to IMAP server...")
	c, err := p.connect()
	if err != nil {
		return attachments, errors.Wrap("could not connect to imap server", err)
	}
	defer c.Logout()
	logrus.WithField("source", p.config.SourceName).Info("Connection successfull.")

	logrus.WithField("source", p.config.SourceName).Info("Loggin in to IMAP server...")
	err = p.login(c)
	if err != nil {
		return attachments, errors.Wrap("could not log in to imap server", err)
	}
	logrus.WithField("source", p.config.SourceName).Info("Login successfull.")

	logrus.WithField("source", p.config.SourceName).Info("Scanning directories...")
	attachments, err = p.scanDirs(c)
	if err != nil {
		return attachments, errors.Wrap("error scanning directories", err)
	}
	logrus.WithField("source", p.config.SourceName).Info("Scanning successfull.")

	return attachments, nil
}

func (p *Poller) connect() (*client.Client, error) {
	// TODO: add suport for startTls: https://godoc.org/github.com/emersion/go-imap/client#ex-Client-StartTLS
	// TODO: add non-TLS support
	// TODO: give a tls.Config for supporting ignore certificates and os on...
	return client.DialTLS(fmt.Sprintf("%s:%d", p.config.SourceConfig.Server, p.config.SourceConfig.Port), nil)
}

func (p *Poller) login(c *client.Client) error {
	return c.Login(p.config.SourceConfig.Username, p.config.SourceConfig.Password)
}

func (p *Poller) scanDirs(c *client.Client) ([]*Attachment, error) {
	attachments := []*Attachment{}

	var errs error
	for _, dir := range p.config.SourceDirectories {
		logrus.WithField("source", p.config.SourceName).Info("Scanning directory \"" + dir + "\"...")
		attach, err := p.scanDir(c, dir)
		if err != nil {
			errs = errors.NewMultiError(errs, err)
		}
		attachments = append(attachments, attach...)
		logrus.WithField("source", p.config.SourceName).Info("Directory \"" + dir + "\" done.")
	}
	return attachments, errs
}

func (p *Poller) scanDir(c *client.Client, dir string) ([]*Attachment, error) {
	attachments := []*Attachment{}

	_, err := c.Select(dir, false)
	if err != nil {
		logrus.WithError(err).Errorf("Could not open directory \"%s\".", dir)
		return attachments, err
	}

	unreadCrit := imap.NewSearchCriteria()
	unreadCrit.WithoutFlags = []string{imap.SeenFlag}
	ids, err := c.Search(unreadCrit)
	if err != nil {
		logrus.WithError(err).Errorf("Could not retreive unread messages in directory \"%s\".", dir)
		return attachments, err
	}
	if len(ids) == 0 {
		logrus.WithField("source", p.config.DestinationConfig.Name).Infof("No unread messages in directory \"%s\".", dir)
		return attachments, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{
		section.FetchItem(),
	}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, items, messages)
		close(done)
	}()

	var errs error
	for msg := range messages {
		attach, err := p.parseMessage(msg, section)
		if err != nil {
			logrus.WithError(err).Error("Could not parse message.")
			errs = errors.NewMultiError(errs, err)
		}
		attachments = append(attachments, attach...)
		// TODO: Add support to optionally expunge mail after upload
	}

	if errs = errors.NewMultiError(errs, <-done); errs != nil {
		logrus.WithError(err).Error("Error fetching mail.")
		return attachments, err
	}
	return attachments, nil
}

func (p *Poller) parseMessage(raw *imap.Message, section *imap.BodySectionName) ([]*Attachment, error) {
	body := raw.GetBody(section)
	msg, err := message.Read(body)
	if err != nil {
		if message.IsUnknownCharset(err) {
			logrus.Warnf("Unknown encoding of message \"%du\".", raw.Uid)
		} else {
			return nil, errors.Wrap("could not read message", err)
		}
	}

	attachments := []*Attachment{}
	if mr := msg.MultipartReader(); mr != nil {
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, errors.Wrap("error reading next part of multipart message", err)
			}

			attach, err := p.parseMsgPart(part)
			if err != nil {
				return nil, errors.Wrap("error parsing part of multipart message", err)
			}
			if attach != nil {
				attachments = append(attachments, attach)
			}
		}
	} else {
		logrus.Warnf("Message \"%du\" is not multipart.", raw.Uid)
	}

	return attachments, nil
}

func (p *Poller) parseMsgPart(part *message.Entity) (*Attachment, error) {
	disp, params, err := part.Header.ContentDisposition()
	if err != nil {
		// Has no content disposition
		// TODO: Maybe add support for parsing email content and adding it as a text file
		// This is not an error, just skip it
		logrus.WithField("source", p.config.DestinationConfig.Name).Debug("Message part is has no Content-Disposition header.")
		return nil, nil
	}
	if disp == "attachment" {
		// TODO: is this robust? Do attachments always have the content disposition set?
		filename, ok := params["filename"]
		if !ok {
			// TODO: Can also be in "Content-Type" header under parameter "name"
			// TODO: Use a fallback random name or something
			//      (maybe, this would require reconstructing the file extension from mime type)
			return nil, errors.New("unable to handle attachment without filename in header")
		}
		encoding := part.Header.Get("Content-Transfer-Encoding")
		if encoding == "" {
			return nil, errors.New("unable to handle attachment without \"Content-Transfer-Encoding\" header")
		}

		// Note go-message already does base64 decoding where necessary
		content, err := ioutil.ReadAll(part.Body)
		if err != nil {
			return nil, errors.Wrap("error reading body of message part", err)
		}

		attachment := &Attachment{
			Filename: filename,
			Content:  content,
			DestinationInfo: &DestinationInfo{
				Config:    p.config.DestinationConfig,
				Directory: p.config.DestinationDirectory,
			},
		}
		logrus.WithField("source", p.config.DestinationConfig.Name).Infof("Successully downloaded file \"%s\".", filename)

		return attachment, nil
	}
	return nil, nil
}
