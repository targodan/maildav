package maildav

import (
	"fmt"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/targodan/go-errors"
)

type IMAPClient interface {
	Login() error
	Unlock()

	Select(name string, readOnly bool) (*imap.MailboxStatus, error)
	Search(criteria *imap.SearchCriteria) (seqNums []uint32, err error)
	Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error
}

type imapClient struct {
	c   *client.Client
	cfg *SourceConfig
	cp  *ConnectionPool
}

func NewIMAPClient(cfg *SourceConfig, cp *ConnectionPool) (IMAPClient, error) {
	var err error
	c := &imapClient{
		cfg: cfg,
		cp:  cp,
	}

	c.c, err = client.DialTLS(fmt.Sprintf("%s:%d", c.cfg.Server, c.cfg.Port), nil)
	if err != nil {
		return nil, errors.Wrap("could not connect to imap server", err)
	}

	return c, nil
}

func (c *imapClient) Unlock() {
    if c.cp != nil {
        c.cp.Unlock(c.cfg)
    }
}

func (c *imapClient) Login() error {
	return c.c.Login(c.cfg.Username, c.cfg.Password)
}

func (c *imapClient) Select(name string, readOnly bool) (*imap.MailboxStatus, error) {
	return c.c.Select(name, readOnly)
}

func (c *imapClient) Search(criteria *imap.SearchCriteria) (seqNums []uint32, err error) {
	return c.c.Search(criteria)
}

func (c *imapClient) Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	return c.c.Fetch(seqset, items, ch)
}
