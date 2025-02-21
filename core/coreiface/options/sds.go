package options

type SdsSettings struct {
	PrivKey  string
	OnlyHash bool
}

type (
	SdsOption func(*SdsSettings) error
)

func SdsOptions(opts ...SdsOption) (*SdsSettings, error) {
	options := &SdsSettings{}

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

// HashOnly will make the adder calculate data hash without storing it in the
// blockstore or announcing it to the network
func (sdsOpts) HashOnly(hashOnly bool) SdsOption {
	return func(settings *SdsSettings) error {
		settings.OnlyHash = hashOnly
		return nil
	}
}
