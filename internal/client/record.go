package client

func (r *Record) IDfromProviderData(providerData any) {
	if pphProviderData, ok := providerData.(PPHProviderData); ok {
		r.ID = pphProviderData.ID
	}
}
