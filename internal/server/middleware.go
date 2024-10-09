package server

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v2"
	"github.com/kataras/hcaptcha"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/negroni"
)

type Limiter struct {
	mutex sync.Mutex
	cache *ttlcache.Cache
	ttl   time.Duration
}

func NewLimiter(ttl time.Duration) *Limiter {
	cache := ttlcache.NewCache()
	cache.SkipTTLExtensionOnHit(true)
	return &Limiter{
		cache: cache,
		ttl:   ttl,
	}
}

func (l *Limiter) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if l.ttl <= 0 {
		next.ServeHTTP(w, r)
		return
	}

	clintIP := getClientIPFromRequest(w, r)
	if clintIP == "" {
		return
	}
	l.mutex.Lock()
	if l.limitByKey(w, clintIP) {
		l.mutex.Unlock()
		return
	}
	l.cache.SetWithTTL(clintIP, true, l.ttl)
	l.mutex.Unlock()

	next.ServeHTTP(w, r)
	if w.(negroni.ResponseWriter).Status() != http.StatusOK {
		l.cache.Remove(clintIP)
		return
	}
	log.WithFields(log.Fields{
		"clientIP": clintIP,
	}).Info("Maximum request limit has been reached")
}

func (l *Limiter) limitByKey(w http.ResponseWriter, key string) bool {
	if _, ttl, err := l.cache.GetWithTTL(key); err == nil {
		errMsg := fmt.Sprintf("You have exceeded the rate limit. Please wait %s before you try again", ttl.Round(time.Second))
		renderJSON(w, claimResponse{Message: errMsg}, http.StatusTooManyRequests)
		return true
	}
	return false
}

func getClientIPFromRequest(w http.ResponseWriter, r *http.Request) string {
	remoteIP := r.Header.Get("X-Real-IP")
	if remoteIP != "" {
		return remoteIP
	}
	remoteIP = r.Header.Get("CF-Connecting-IP")
	if remoteIP != "" {
		return remoteIP
	}
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}
	if remoteIP == "" {
		errMsg := "The client address is missing"
		renderJSON(w, claimResponse{Message: errMsg}, http.StatusInternalServerError)
	}
	return remoteIP
}

type Captcha struct {
	client *hcaptcha.Client
	secret string
}

func NewCaptcha(hcaptchaSiteKey, hcaptchaSecret string) *Captcha {
	client := hcaptcha.New(hcaptchaSecret)
	client.SiteKey = hcaptchaSiteKey
	return &Captcha{
		client: client,
		secret: hcaptchaSecret,
	}
}

func (c *Captcha) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if c.secret == "" {
		next.ServeHTTP(w, r)
		return
	}

	response := c.client.VerifyToken(r.Header.Get("h-captcha-response"))
	if !response.Success {
		renderJSON(w, claimResponse{Message: "Captcha verification failed, please try again"}, http.StatusTooManyRequests)
		return
	}

	next.ServeHTTP(w, r)
}
