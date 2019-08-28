package maildav

import (
	"sync"

	"github.com/tarent/logrus"
	"github.com/targodan/go-errors"
)

type ConnectionPool struct {
	mapLock     sync.Locker
	connections map[string]IMAPClient
	locks       map[string]sync.Locker
}

func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		mapLock:     &sync.Mutex{},
		connections: make(map[string]IMAPClient),
		locks:       make(map[string]sync.Locker),
	}
}

func (cp *ConnectionPool) getOrConnect(cfg *SourceConfig) (IMAPClient, error) {
	cp.mapLock.Lock()
	defer cp.mapLock.Unlock()

	c, exists := cp.connections[cfg.Name]
	if exists {
		logrus.WithField("source", cfg.Name).Info("Reusing connection.")
	} else {
		// TODO: add suport for startTls: https://godoc.org/github.com/emersion/go-imap/client#ex-Client-StartTLS
		// TODO: add non-TLS support
		// TODO: give a tls.Config for supporting ignore certificates and os on...
		var err error

		logrus.WithField("source", cfg.Name).Info("Connecting to IMAP server...")

		c, err = NewIMAPClient(cfg, cp)
		if err != nil {
			return nil, errors.Wrap("could not connect to imap server", err)
		}

		logrus.WithField("source", cfg.Name).Info("Connection successfull.")

		logrus.WithField("source", cfg.Name).Info("Logging in to IMAP server...")
		err = c.Login()
		if err != nil {
			return nil, errors.Wrap("could not log in to imap server", err)
		}
		logrus.WithField("source", cfg.Name).Info("Login successfull.")

		cp.connections[cfg.Name] = c
		cp.locks[cfg.Name] = &sync.Mutex{}
	}

	return c, nil
}

func (cp *ConnectionPool) ConnectAndLock(cfg *SourceConfig) (IMAPClient, error) {
	c, err := cp.getOrConnect(cfg)
	if err != nil {
		return nil, err
	}
	logrus.WithField("source", cfg.Name).Debug("Locking connection...")
	cp.locks[cfg.Name].Lock()
	logrus.WithField("source", cfg.Name).Debug("Lock aquired.")

	return c, nil
}

func (cp *ConnectionPool) Unlock(cfg *SourceConfig) {
	lock, ok := cp.locks[cfg.Name]
	if ok {
		lock.Unlock()
	}
}
