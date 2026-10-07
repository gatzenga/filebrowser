package fbhttp

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
	"github.com/filebrowser/filebrowser/v2/users"
)

func signToken(t *testing.T, perm users.Permissions, key []byte) string {
	t.Helper()
	claims := &authToken{
		User: userInfo{ID: 1, Username: "u", Perm: perm},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

// scopedUserStorage returns a storage whose single user (ID 1) is scoped to
// userScope through a symlink-confining ScopedFs (via customFSUser), mirroring
// production. Used by the symlink scope-escape regression tests below.
func scopedUserStorage(t *testing.T, userScope string, perm users.Permissions, key []byte) *storage.Storage {
	t.Helper()
	db, err := storm.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatalf("failed to get storage: %v", err)
	}
	if err := st.Users.Save(&users.User{Username: "u", Password: "pw", Perm: perm}); err != nil {
		t.Fatalf("failed to save user: %v", err)
	}
	if err := st.Settings.Save(&settings.Settings{Key: key}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}
	st.Users = &customFSUser{
		Store: st.Users,
		fs:    afero.NewBasePathFs(afero.NewOsFs(), userScope),
	}
	return st
}

type customFSUser struct {
	users.Store
	fs afero.Fs
	// followExternal mirrors Server.FollowExternalSymlinks: when set, the
	// provided fs is used as-is (a bare BasePathFs that follows symlinks);
	// otherwise it is wrapped in a symlink-confining ScopedFs.
	followExternal bool
}

func (cu *customFSUser) Get(baseScope string, followExternalSymlinks bool, id interface{}) (*users.User, error) {
	user, err := cu.Store.Get(baseScope, followExternalSymlinks, id)
	if err != nil {
		return nil, err
	}
	// Inject a filesystem rooted at the test's temp scope, standing in for the
	// one users.User.Clean would build in production.
	if cu.followExternal {
		user.Fs = cu.fs
	} else {
		user.Fs = files.NewScopedFs(cu.fs, "/")
	}

	return user, nil
}
