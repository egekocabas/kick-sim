package studio

import (
	"context"
	"fmt"
	"os"

	"github.com/egekocabas/kick-sim/internal/config"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func (backend *Backend) PublicKey(context.Context) (kickopenapi.PublicKey, error) {
	path := config.ResolvePath(backend.service.WorkspaceRoot(), backend.service.Configuration().Signing.PublicKey)
	data, err := workspace.ReadPublicKeyPEM(path)
	if err != nil {
		return kickopenapi.PublicKey{}, fmt.Errorf("read public key: %w", err)
	}
	return kickopenapi.PublicKey{Path: path, PEM: string(data)}, nil
}

func (backend *Backend) KeyInfo(context.Context) (kickopenapi.KeyInfo, error) {
	configuration := backend.service.Configuration()
	publicPath := config.ResolvePath(backend.service.WorkspaceRoot(), configuration.Signing.PublicKey)
	privatePath := config.ResolvePath(backend.service.WorkspaceRoot(), configuration.Signing.PrivateKey)
	publicKey, matching, err := workspace.InspectKeyPair(privatePath, publicPath)
	if err != nil {
		return kickopenapi.KeyInfo{}, err
	}
	fingerprint, err := signing.Fingerprint(publicKey)
	if err != nil {
		return kickopenapi.KeyInfo{}, err
	}
	info, err := os.Stat(publicPath)
	if err != nil {
		return kickopenapi.KeyInfo{}, err
	}
	return kickopenapi.KeyInfo{
		Path: publicPath, Algorithm: "RSA", Bits: publicKey.N.BitLen(), Fingerprint: fingerprint,
		ModifiedAt: info.ModTime().UTC(), MatchingPrivateKey: matching,
	}, nil
}

func (backend *Backend) RotateKey(ctx context.Context) (kickopenapi.KeyInfo, error) {
	if err := workspace.RotateKeys(backend.service.WorkspaceRoot()); err != nil {
		return kickopenapi.KeyInfo{}, err
	}
	return backend.KeyInfo(ctx)
}
