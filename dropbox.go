package wsh2s

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"strings"

	"github.com/aead/chacha20"
	"github.com/tj/go-dropbox"
	"github.com/tj/go-dropy"
)

type dropboxer struct {
	client *dropy.Client
	aead   cipher.AEAD
}

func newDropbox(accessToken string, secretKey string) (*dropboxer, error) {
	var sk [32]byte
	_, err := base64.NewDecoder(base64.StdEncoding, strings.NewReader(secretKey)).Read(sk[:])
	if err != nil {
		return nil, err
	}
	return &dropboxer{
		client: dropy.New(dropbox.New(dropbox.NewConfig(accessToken))),
		aead:   chacha20.NewChaCha20Poly1305(&sk),
	}, nil
}

func (d *dropboxer) SaveFile(path string, plaintext []byte) error {
	ciphertext := make([]byte, len(plaintext)+chacha20.TagSize+chacha20.NonceSize)
	nonce := ciphertext[:chacha20.NonceSize]
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	d.aead.Seal(ciphertext[chacha20.NonceSize:chacha20.NonceSize], nonce, plaintext, nil)
	return d.client.Upload(path, bytes.NewReader(ciphertext))
}

func (d *dropboxer) LoadFile(path string) ([]byte, error) {
	// if not use Read, check "path/not_found/" in error
	ciphertext, err := d.client.Read(path)
	if _, ok := err.(*os.PathError); ok {
		return nil, os.ErrNotExist
	}

	ltxt := len(ciphertext) - chacha20.TagSize - chacha20.NonceSize
	if ltxt <= 0 {
		return nil, os.ErrNotExist
	}

	plaintext := make([]byte, ltxt)
	return d.aead.Open(plaintext[:0], ciphertext[:chacha20.NonceSize], ciphertext[chacha20.NonceSize:], nil)
}

func (d *dropboxer) SavePlainFile(path string, plaintext []byte) error {
	return d.client.Upload(path, bytes.NewReader(plaintext))
}

func (d *dropboxer) LoadPlainFile(path string) (plaintext []byte, err error) {
	// if not use Read, check "path/not_found/" in error
	plaintext, err = d.client.Read(path)
	if _, ok := err.(*os.PathError); ok {
		err = os.ErrNotExist
	}
	return
}
