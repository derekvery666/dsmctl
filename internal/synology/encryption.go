package synology

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" // DSM's CryptoJS/OpenSSL-compatible transport KDF requires MD5.
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	encryptionAPIName = "SYNO.API.Encryption"
	randomKeyLength   = 501
	randomKeyAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ~!@#$%^&*()_+-/"
)

type encryptionInfo struct {
	CipherKey   string `json:"cipherkey"`
	CipherToken string `json:"ciphertoken"`
	PublicKey   string `json:"public_key"`
	ServerTime  int64  `json:"server_time"`
}

type encryptedBundle struct {
	RSA string `json:"rsa"`
	AES string `json:"aes"`
}

func (c *Client) encodeJSONParametersLocked(ctx context.Context, values map[string]any, encryptedNames []string) (url.Values, error) {
	parameters := make(url.Values, len(values))
	encryptTransport := len(encryptedNames) != 0 && c.baseURL.Scheme != "https"
	encrypted := make(map[string]any, len(encryptedNames))
	encryptedSet := make(map[string]struct{}, len(encryptedNames))
	for _, name := range encryptedNames {
		if _, duplicate := encryptedSet[name]; duplicate {
			return nil, fmt.Errorf("encrypted parameter %q appears more than once", name)
		}
		value, found := values[name]
		if !found {
			return nil, fmt.Errorf("encrypted parameter %q is missing", name)
		}
		encryptedSet[name] = struct{}{}
		encrypted[name] = value
	}
	for name, value := range values {
		if _, secret := encryptedSet[name]; secret && encryptTransport {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode JSON parameter %q: %w", name, err)
		}
		parameters.Set(name, string(encoded))
	}
	if !encryptTransport {
		return parameters, nil
	}
	info, err := c.encryptionInfoLocked(ctx)
	if err != nil {
		return nil, err
	}
	bundle, err := encryptDSMFields(info, encrypted, encryptedNames, rand.Reader)
	if err != nil {
		return nil, err
	}
	encodedBundle, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("encode DSM encrypted parameter bundle: %w", err)
	}
	parameters.Set(info.CipherKey, string(encodedBundle))
	return parameters, nil
}

func (c *Client) encryptionInfoLocked(ctx context.Context) (encryptionInfo, error) {
	if err := c.ensureAPIsLocked(ctx, encryptionAPIName); err != nil {
		return encryptionInfo{}, fmt.Errorf("discover DSM parameter-encryption API: %w", err)
	}
	info, _ := c.target.API(encryptionAPIName)
	parameters := url.Values{
		"api":     {encryptionAPIName},
		"version": {"1"},
		"method":  {"getinfo"},
		"format":  {"module"},
		"_sid":    {c.sid},
	}
	if c.synoToken != "" {
		parameters.Set("SynoToken", c.synoToken)
	}
	data, err := c.requestLocked(ctx, info.Path, parameters, encryptionAPIName, "getinfo")
	if err != nil {
		return encryptionInfo{}, fmt.Errorf("get DSM parameter-encryption key: %w", err)
	}
	var result encryptionInfo
	if err := json.Unmarshal(data, &result); err != nil {
		return encryptionInfo{}, fmt.Errorf("decode DSM parameter-encryption key: %w", err)
	}
	if result.CipherKey == "" || result.CipherToken == "" || result.PublicKey == "" || result.ServerTime <= 0 {
		return encryptionInfo{}, fmt.Errorf("DSM returned incomplete parameter-encryption metadata")
	}
	return result, nil
}

func encryptDSMFields(info encryptionInfo, fields map[string]any, fieldOrder []string, random io.Reader) (encryptedBundle, error) {
	publicKey, err := parseDSMRSAPublicKey(info.PublicKey)
	if err != nil {
		return encryptedBundle{}, err
	}
	if publicKey.Size()-11 < randomKeyLength {
		return encryptedBundle{}, fmt.Errorf("DSM RSA key is too small for the required transport key")
	}
	transportKey, err := randomAlphabetString(random, randomKeyLength, randomKeyAlphabet)
	if err != nil {
		return encryptedBundle{}, fmt.Errorf("generate DSM transport key: %w", err)
	}
	rsaCiphertext, err := rsa.EncryptPKCS1v15(random, publicKey, []byte(transportKey))
	if err != nil {
		return encryptedBundle{}, fmt.Errorf("encrypt DSM transport key: %w", err)
	}

	parts := []string{jsEncodeURIComponent(info.CipherToken) + "=" + jsEncodeURIComponent(strconv.FormatInt(info.ServerTime, 10))}
	seen := make(map[string]struct{}, len(fieldOrder))
	for _, name := range fieldOrder {
		if _, duplicate := seen[name]; duplicate {
			return encryptedBundle{}, fmt.Errorf("encrypted field %q appears more than once", name)
		}
		value, found := fields[name]
		if !found {
			return encryptedBundle{}, fmt.Errorf("encrypted field %q is missing", name)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return encryptedBundle{}, fmt.Errorf("encode encrypted field %q: %w", name, err)
		}
		parts = append(parts, jsEncodeURIComponent(name)+"="+jsEncodeURIComponent(string(encoded)))
		seen[name] = struct{}{}
	}
	// Reject unlisted secret fields so a caller cannot accidentally omit one
	// from the encrypted payload while leaving it in memory-only input.
	if len(seen) != len(fields) {
		unlisted := make([]string, 0, len(fields)-len(seen))
		for name := range fields {
			if _, included := seen[name]; !included {
				unlisted = append(unlisted, name)
			}
		}
		sort.Strings(unlisted)
		return encryptedBundle{}, fmt.Errorf("encrypted fields were not ordered: %s", strings.Join(unlisted, ", "))
	}
	aesCiphertext, err := cryptoJSPassphraseEncrypt(random, []byte(strings.Join(parts, "&")), []byte(transportKey))
	if err != nil {
		return encryptedBundle{}, err
	}
	return encryptedBundle{
		RSA: base64.StdEncoding.EncodeToString(rsaCiphertext),
		AES: aesCiphertext,
	}, nil
}

func parseDSMRSAPublicKey(value string) (*rsa.PublicKey, error) {
	// With format=module DSM returns the RSA modulus as hexadecimal and the
	// WebUI supplies exponent 0x10001. Other callers can receive a base64 DER
	// SubjectPublicKeyInfo value, so accept both advertised representations.
	if modulus, ok := new(big.Int).SetString(value, 16); ok && len(value)%2 == 0 && modulus.BitLen() >= 2048 {
		return &rsa.PublicKey{N: modulus, E: 65537}, nil
	}
	der, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode DSM encryption public key: %w", err)
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse DSM encryption public key: %w", err)
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("DSM encryption public key is not RSA")
	}
	return publicKey, nil
}

func cryptoJSPassphraseEncrypt(random io.Reader, plaintext, passphrase []byte) (string, error) {
	salt := make([]byte, 8)
	if _, err := io.ReadFull(random, salt); err != nil {
		return "", fmt.Errorf("generate DSM AES salt: %w", err)
	}
	derived := make([]byte, 0, 48)
	var previous []byte
	for len(derived) < 48 {
		hash := md5.New()
		hash.Write(previous)
		hash.Write(passphrase)
		hash.Write(salt)
		previous = hash.Sum(nil)
		derived = append(derived, previous...)
	}
	block, err := aes.NewCipher(derived[:32])
	if err != nil {
		return "", fmt.Errorf("initialize DSM AES cipher: %w", err)
	}
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for index := len(plaintext); index < len(padded); index++ {
		padded[index] = byte(padding)
	}
	cipher.NewCBCEncrypter(block, derived[32:48]).CryptBlocks(padded, padded)
	formatted := make([]byte, 0, 16+len(padded))
	formatted = append(formatted, []byte("Salted__")...)
	formatted = append(formatted, salt...)
	formatted = append(formatted, padded...)
	return base64.StdEncoding.EncodeToString(formatted), nil
}

func randomAlphabetString(random io.Reader, length int, alphabet string) (string, error) {
	if length < 0 || len(alphabet) == 0 || len(alphabet) > 256 {
		return "", fmt.Errorf("invalid random alphabet request")
	}
	result := make([]byte, length)
	buffer := make([]byte, 1)
	limit := 256 - (256 % len(alphabet))
	for index := 0; index < length; {
		if _, err := io.ReadFull(random, buffer); err != nil {
			return "", err
		}
		if int(buffer[0]) >= limit {
			continue
		}
		result[index] = alphabet[int(buffer[0])%len(alphabet)]
		index++
	}
	return string(result), nil
}

func jsEncodeURIComponent(value string) string {
	const hexadecimal = "0123456789ABCDEF"
	var builder strings.Builder
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune("-_.!~*'()", rune(character)) {
			builder.WriteByte(character)
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(hexadecimal[character>>4])
		builder.WriteByte(hexadecimal[character&0x0f])
	}
	return builder.String()
}
