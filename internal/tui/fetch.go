package tui

import (
	"github.com/dvdsvds/carty/internal/cache"
	"github.com/dvdsvds/carty/internal/catalog"
	"github.com/dvdsvds/carty/internal/fetch"
)

// fetchIndexBytes fetches the data repository's index.json, caching it on
// success. If the network fetch fails, it falls back to whatever is
// already cached (fromCache=true) so browsing/search keeps working
// offline, per the README's caching section.
func fetchIndexBytes(cacheDir string) (data []byte, fromCache bool, err error) {
	data, err = fetch.Get(indexURL)
	if err == nil {
		_ = cache.SaveIndex(cacheDir, data)
		return data, false, nil
	}

	cached, cacheErr := cache.LoadIndex(cacheDir)
	if cacheErr != nil {
		return nil, false, err
	}
	return cached, true, nil
}

// fetchItemBody fetches an item's config file body, caching it on success
// and falling back to the cache on network failure.
func fetchItemBody(cacheDir string, item catalog.Item) (string, error) {
	data, err := fetch.Get(rawBaseURL + "files/" + item.FilePath)
	if err == nil {
		_ = cache.SaveFile(cacheDir, item.FilePath, data)
		return catalog.ExtractBody(data), nil
	}

	cached, cacheErr := cache.LoadFile(cacheDir, item.FilePath)
	if cacheErr != nil {
		return "", err
	}
	return catalog.ExtractBody(cached), nil
}
