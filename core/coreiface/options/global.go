package options

type ApiSettings struct {
	Offline     bool
	FetchBlocks bool
	SdsFetcher  SdsFetcher
}

type ApiOption func(*ApiSettings) error

func ApiOptions(opts ...ApiOption) (*ApiSettings, error) {
	options := &ApiSettings{
		Offline:     false,
		FetchBlocks: true,
		SdsFetcher:  nil,
	}

	return ApiOptionsTo(options, opts...)
}

func ApiOptionsTo(options *ApiSettings, opts ...ApiOption) (*ApiSettings, error) {
	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type apiOpts struct{}

var Api apiOpts

func (apiOpts) Offline(offline bool) ApiOption {
	return func(settings *ApiSettings) error {
		settings.Offline = offline
		return nil
	}
}

// FetchBlocks when set to false prevents api from fetching blocks from the
// network while allowing other services such as IPNS to still be online
func (apiOpts) FetchBlocks(fetch bool) ApiOption {
	return func(settings *ApiSettings) error {
		settings.FetchBlocks = fetch
		return nil
	}
}

type SdsFetcher interface {
	Download(fileHash string) ([]byte, error)
	Upload(fileData []byte) (string, error)
}

// sds
//
// PoC
func (apiOpts) SdsFetcher(fetcher SdsFetcher) ApiOption {
	return func(settings *ApiSettings) error {
		settings.SdsFetcher = fetcher
		return nil
	}
}
