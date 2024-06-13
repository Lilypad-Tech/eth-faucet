package server

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/negroni"

	"github.com/chainflag/eth-faucet/internal/chain"
	"github.com/chainflag/eth-faucet/web"
)

type Server struct {
	chain.TxBuilder
	cfg *Config
}

func NewServer(builder chain.TxBuilder, cfg *Config) *Server {
	return &Server{
		TxBuilder: builder,
		cfg:       cfg,
	}
}

func (s *Server) setupRouter() *http.ServeMux {
	router := http.NewServeMux()
	router.Handle("/", http.FileServer(web.Dist()))
	limiter := NewLimiter(s.cfg.proxyCount, time.Duration(s.cfg.interval)*time.Minute)
	hcaptcha := NewCaptcha(s.cfg.hcaptchaSiteKey, s.cfg.hcaptchaSecret)
	router.Handle("/api/claim", negroni.New(limiter, hcaptcha, negroni.Wrap(s.handleClaim())))
	router.Handle("/api/claim_eth_with_token", negroni.New(limiter, hcaptcha, negroni.Wrap(s.handleClaimEthWithToken())))

	router.Handle("/api/info", s.handleInfo())

	return router
}

func (s *Server) Run() {
	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger())
	n.UseHandler(s.setupRouter())
	log.Infof("Starting http server %d", s.cfg.httpPort)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(s.cfg.httpPort), n))
}

func (s *Server) handleClaim() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.NotFound(w, r)
			return
		}

		// The error always be nil since it has already been handled in limiter
		address, _ := readAddress(r)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		//claim eth
		// bigHundred := big.NewInt(1000)
		// payout := chain.EtherToWei(int64(s.cfg.payout))
		// payout.Div(payout, bigHundred)
		// txHash, err := s.Transfer(ctx, address, payout)
		// if err != nil {
		// 	log.WithError(err).Error("Failed to send transaction")
		// 	renderJSON(w, claimResponse{Message: err.Error()}, http.StatusInternalServerError)
		// 	return
		// }
		// time.Sleep(2 * time.Second)
		tokenTxHash, err := s.TransferTokens(ctx, address, chain.EtherToWei(int64(s.cfg.tokenPayout)))
		if err != nil {
			log.WithError(err).Error("Failed to send transaction")
			renderJSON(w, claimResponse{Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		log.WithFields(log.Fields{
			// "txHash":      txHash,
			"tokenTxHash": tokenTxHash,
			"address":     address,
		}).Info("Transaction sent successfully")
		// resp := claimResponse{Message: fmt.Sprintf("Txhash: %s, TokenTxhash: %s", txHash, tokenTxHash)}
		resp := claimResponse{Message: fmt.Sprintf("TokenTxhash: %s", tokenTxHash)}
		renderJSON(w, resp, http.StatusOK)
	}
}
func (s *Server) handleClaimEthWithToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.NotFound(w, r)
			return
		}

		// The error always be nil since it has already been handled in limiter
		address, _ := readAddress(r)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		bigHundred := big.NewInt(1000)
		payout := chain.EtherToWei(int64(s.cfg.payout))
		payout.Div(payout, bigHundred)
		txHash, err := s.Transfer(ctx, address, payout)
		if err != nil {
			log.WithError(err).Error("Failed to send transaction")
			renderJSON(w, claimResponse{Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		time.Sleep(2 * time.Second)
		tokenTxHash, err := s.TransferTokens(ctx, address, chain.EtherToWei(int64(s.cfg.tokenPayout)))
		if err != nil {
			log.WithError(err).Error("Failed to send transaction")
			renderJSON(w, claimResponse{Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		log.WithFields(log.Fields{
			"txHash":      txHash,
			"tokenTxHash": tokenTxHash,
			"address":     address,
		}).Info("Transaction sent successfully")
		resp := claimResponse{Message: fmt.Sprintf("Txhash: %s, TokenTxhash: %s", txHash, tokenTxHash)}
		renderJSON(w, resp, http.StatusOK)
	}
}
func (s *Server) handleInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.NotFound(w, r)
			return
		}
		renderJSON(w, infoResponse{
			Account:         s.Sender().String(),
			Network:         s.cfg.network,
			Symbol:          s.cfg.symbol,
			Payout:          strconv.Itoa(s.cfg.payout),
			HcaptchaSiteKey: s.cfg.hcaptchaSiteKey,
		}, http.StatusOK)
	}
}
