package server

type Config struct {
	network         string
	symbol          string
	httpPort        int
	interval        int
	payout          int
	tokenPayout     int
	proxyCount      int
	hcaptchaSiteKey string
	hcaptchaSecret  string
}

func NewConfig(network, symbol string, httpPort, interval, payout, tokenPayout, proxyCount int, hcaptchaSiteKey, hcaptchaSecret string) *Config {
	return &Config{
		network:         network,
		symbol:          symbol,
		httpPort:        httpPort,
		interval:        interval,
		payout:          payout,
		tokenPayout:     tokenPayout,
		proxyCount:      proxyCount,
		hcaptchaSiteKey: hcaptchaSiteKey,
		hcaptchaSecret:  hcaptchaSecret,
	}
}
