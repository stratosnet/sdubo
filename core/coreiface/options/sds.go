package options

type SdsSettings struct {
	PrivKey string
}

type (
	SdsOption func(*SdsSettings) error
)

func SdsOptions(opts ...SdsOption) (*SdsSettings, error) {
	options := &SdsSettings{
		PrivKey: "", // TODO
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

type sdsOpts struct{}

var Sds sdsOpts

// PrivKey specifies custom the private key for signing
func (sdsOpts) PrivKey(privKey string) SdsOption {
	return func(settings *SdsSettings) error {
		settings.PrivKey = privKey
		return nil
	}
}
