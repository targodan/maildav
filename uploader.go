package maildav

import (
	"github.com/studio-b12/gowebdav"
	"github.com/tarent/logrus"
	"github.com/targodan/go-errors"
)

type Uploader struct {
}

func (u *Uploader) UploadAttachments(attachments []*Attachment) error {
	if len(attachments) == 0 {
		return nil
	}

	uploads := map[string][]*Attachment{}

	for _, attach := range attachments {
		at, ok := uploads[attach.DestinationInfo.Config.Name]
		if !ok {
			at = make([]*Attachment, 0)
		}
		at = append(at, attach)
		uploads[attach.DestinationInfo.Config.Name] = at
	}

	return u.uploadAttachments(uploads)
}

func (u *Uploader) uploadAttachments(uploads map[string][]*Attachment) error {
	logrus.Info("Uploading attachments...")

	var errs error
	for _, attachments := range uploads {
		cfg := attachments[0].DestinationInfo.Config
		logrus.WithField("dest", cfg.Name).Info("Connecting to destination...")

		c, err := u.connect(cfg)
		if err != nil {
			logrus.WithError(err).Errorf("Error connecting to destination \"%s\".", cfg.Name)
			errs = errors.NewMultiError(errors.Wrap("could not connect to destination", err), errs)
			continue
		}
		logrus.WithField("dest", cfg.Name).Info("Connected successfully.")

		for _, at := range attachments {
			logrus.WithField("dest", cfg.Name).Infof("Uploading file \"%s\"...", at.Filename)
			path := gowebdav.Join(at.DestinationInfo.Directory, at.Filename)
			err = c.Write(path, at.Content, 0644)
			if err != nil {
				logrus.WithError(err).Errorf("Error uploading file \"%s\" to destination \"%s\".", at.Filename, cfg.Name)
				errs = errors.NewMultiError(err, errs)
			}
			logrus.WithField("dest", cfg.Name).Infof("File \"%s\" successfully uploaded.", at.Filename)
		}
	}
	return errs
}

func (u *Uploader) connect(cfg *DestinationConfig) (*gowebdav.Client, error) {
	c := gowebdav.NewClient(cfg.BaseURL, cfg.Username, cfg.Password)
	err := c.Connect()
	if err != nil {
		c = nil
	}
	return c, err
}
