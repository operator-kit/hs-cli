package auth

import "github.com/zalando/go-keyring"

const service = "hs-cli"

func StoreInboxCredentials(appID, appSecret string) error {
	if err := keyring.Set(service, "inbox_app_id", appID); err != nil {
		return err
	}
	return keyring.Set(service, "inbox_app_secret", appSecret)
}

func LoadInboxCredentials() (appID, appSecret string, err error) {
	appID, err = keyring.Get(service, "inbox_app_id")
	if err != nil {
		return "", "", err
	}
	appSecret, err = keyring.Get(service, "inbox_app_secret")
	if err != nil {
		return "", "", err
	}
	return appID, appSecret, nil
}

func DeleteInboxCredentials() error {
	_ = keyring.Delete(service, "inbox_app_id")
	_ = keyring.Delete(service, "inbox_app_secret")
	return nil
}

func StoreDocsAPIKey(key string) error {
	return keyring.Set(service, "docs_api_key", key)
}

func LoadDocsAPIKey() (string, error) {
	return keyring.Get(service, "docs_api_key")
}

func DeleteDocsAPIKey() error {
	_ = keyring.Delete(service, "docs_api_key")
	return nil
}
