package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword, duz metin sifreyi bcrypt ile hashler.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword, duz metin sifrenin hash ile eslesip eslesmedigini soyler.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
