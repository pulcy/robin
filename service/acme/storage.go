package acme

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/juju/errgo"
	"github.com/xenolf/lego/acme"
)

// getPrivateKey loads the private key from the private key path.
// If there is no such file, a new private key is generated.
func (s *acmeService) getPrivateKey() (*rsa.PrivateKey, error) {
	key, err := loadRSAPrivateKey(s.PrivateKeyPath)
	if err == nil {
		return key, nil
	} else if !os.IsNotExist(errgo.Cause(err)) {
		return nil, maskAny(err)
	}

	// private key not found, generate one
	key, err = rsa.GenerateKey(rand.Reader, s.KeyBits)
	if err != nil {
		return nil, maskAny(err)
	}

	if err := saveRSAPrivateKey(key, s.PrivateKeyPath); err != nil {
		return nil, maskAny(err)
	}

	return key, nil
}

// getRegistration reads the registration resource for the registration path.
// If no such file exists, nil is returned.
func (s *acmeService) getRegistration() (*acme.RegistrationResource, error) {
	raw, err := ioutil.ReadFile(s.RegistrationPath)
	if err != nil {
		if os.IsNotExist(errgo.Cause(err)) {
			return nil, nil
		}
		return nil, maskAny(err)
	}

	res := &acme.RegistrationResource{}
	if err := json.Unmarshal(raw, res); err != nil {
		return nil, maskAny(err)
	}

	return res, nil
}

// saveRegistration saves the given registration at the configured path
func (s *acmeService) saveRegistration(res *acme.RegistrationResource) error {
	if err := ensureDirectoryOf(s.RegistrationPath, 0755); err != nil {
		return maskAny(err)
	}

	raw, err := json.Marshal(res)
	if err != nil {
		return maskAny(err)
	}

	if err := ioutil.WriteFile(s.RegistrationPath, raw, 0600); err != nil {
		return maskAny(err)
	}

	return nil
}

// loadRSAPrivateKey loads a PEM-encoded RSA private key from file.
func loadRSAPrivateKey(file string) (*rsa.PrivateKey, error) {
	keyBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, maskAny(err)
	}
	keyBlock, _ := pem.Decode(keyBytes)
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, maskAny(err)
	}
	return key, nil
}

// saveRSAPrivateKey saves a PEM-encoded RSA private key to file.
func saveRSAPrivateKey(key *rsa.PrivateKey, path string) error {
	if err := ensureDirectoryOf(path, 0755); err != nil {
		return maskAny(err)
	}
	pemKey := pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	keyOut, err := os.Create(path)
	if err != nil {
		return maskAny(err)
	}
	keyOut.Chmod(0600)
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pemKey); err != nil {
		return maskAny(err)
	}
	return nil
}

// ensureDirectoryOf creates the directory part of the given file path if needed.
func ensureDirectoryOf(path string, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, perm); err != nil {
		return maskAny(err)
	}
	return nil
}
