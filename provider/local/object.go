package local

import (
	"fmt"
	"io"

	"github.com/convox/praxis/types"
)

func (p *Provider) ObjectFetch(app, key string) (io.ReadCloser, error) {
	if _, err := p.AppGet(app); err != nil {
		return nil, err
	}

	token := fmt.Sprintf("apps/%s/objects/%s", app, key)

	if !p.Exists(token) {
		return nil, fmt.Errorf("no such key: %s", key)
	}

	return p.Read(token)
}

func (p *Provider) ObjectStore(app, key string, r io.Reader, opts types.ObjectStoreOptions) (*types.Object, error) {
	if _, err := p.AppGet(app); err != nil {
		return nil, err
	}

	if key == "" {
		return nil, fmt.Errorf("key must not be blank")
	}

	if err := p.Store(fmt.Sprintf("apps/%s/objects/%s", app, key), r); err != nil {
		return nil, err
	}

	return &types.Object{Key: key}, nil
}