package main

import (
	"net/http"
	"strings"
	"sync"

	"github.com/xenolf/lego/acme"
)

//  github.com/dkumor/acmewrapper/configuration.go++
//  type Config struct {
//	  ChallengeProvider acme.TLSSNI01ChallengeProvider
//  }

//  github.com/dkumor/acmewrapper/acme.go ++--
//	w.client.ExcludeChallenges([]acme.Challenge{acme.TLSSNI01, acme.DNS01})
//	w.client.SetChallengeProvider(acme.HTTP01, w.Config.TLSSNI01ChallengeProvider)
//	//	w.client.SetTLSAddress(w.Config.Address)
type wrapperChallengeProvider struct {
	data *challengeProviderData
	mu   sync.Mutex
}

type challengeProviderData struct {
	domain, token, keyAuth string
}

// Present sets up the challenge domain thru SNI. Part of acme.ChallengeProvider interface
func (c *wrapperChallengeProvider) Present(domain, token, keyAuth string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	logf("[acmewrapper] Present for %s", domain)
	c.data = &challengeProviderData{domain, token, keyAuth}
	return nil
}

func (c *wrapperChallengeProvider) challengeHanlder(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.data == nil {
		logf("[INFO] Received request for domain %s with method %s after end", r.Host, r.Method)
		w.WriteHeader(http.StatusNotFound)
	} else if strings.HasPrefix(r.Host, c.data.domain) && r.Method == "GET" && acme.HTTP01ChallengePath(c.data.token) == r.URL.Path {
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte(c.data.keyAuth))
		logf("[INFO][%s] Served key authentication", c.data.domain)
	} else {
		logf("[INFO] Received request for domain %s with method %s", r.Host, r.Method)
		w.Write([]byte("TEST"))
	}
}

// CleanUp removes the challenge domain from SNI. Part of acme.ChallengeProvider interface
func (c *wrapperChallengeProvider) CleanUp(domain, token, keyAuth string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	logf("[acmewrapper] CleanUp for %s\n", domain)
	c.data = nil
	return nil
}
