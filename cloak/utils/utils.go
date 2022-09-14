package utils

import (
	"fmt"

	"cuelang.org/go/cue"
	"github.com/dagger/cloak/sdk/go/dagger"
	"go.dagger.io/dagger/compiler"
	"go.dagger.io/dagger/pkg"
)

var fsIDPath = cue.MakePath(
	cue.Str("$dagger"),
	cue.Str("fs"),
	cue.Hid("_id", pkg.DaggerPackage),
)

var secretIDPath = cue.MakePath(
	cue.Str("$dagger"),
	cue.Str("secret"),
	cue.Hid("_id", pkg.DaggerPackage),
)

func GetFSId(v *compiler.Value) (dagger.FSID, error) {
	var fsid dagger.FSID
	if !v.LookupPath(fsIDPath).IsConcrete() {
		return fsid, fmt.Errorf("invalid FS at path %q: FS is not set", v.Path())
	}
	id, err := v.LookupPath(fsIDPath).String()
	if err != nil {
		return fsid, fmt.Errorf("invalid FS at path %q: %w", v.Path(), err)
	}
	fsid = dagger.FSID(id)
	return fsid, nil
}

func GetSecretId(v *compiler.Value) (dagger.SecretID, error) {
	var secretid dagger.SecretID
	if !v.LookupPath(secretIDPath).IsConcrete() {
		return secretid, fmt.Errorf("invalid Secret at path %q: Secret is not set", v.Path())
	}
	id, err := v.LookupPath(secretIDPath).String()
	if err != nil {
		return secretid, fmt.Errorf("invalid Secret at path %q: %w", v.Path(), err)
	}
	secretid = dagger.SecretID(id)
	return secretid, nil
}

func NewFS(id dagger.FSID) *compiler.Value {
	v := compiler.NewValue()

	if err := v.FillPath(fsIDPath, id); err != nil {
		panic(err)
	}

	return v
}
