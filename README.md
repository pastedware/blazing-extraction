# blazing extraction
blazingly fast multithreaded browser extraction made in golang

CHECKLIST:
- [x] Password Extraction
- [x] Cookie Extraction
- [ ] Credit Card Extraction
- [ ] Crypto Wallet Extraction
- [ ] Token Extraction
- [ ] History Extraction


# How to use? 
download the precompiled binary in the release folder (currently only x86_64, cuz i aint cross compiling, build it yourself)

### decrypt both passwords and cookies
```
.\browser.exe
```

### decrypt passwords only
```
.\browser.exe --passwords
```

### decrypt cookies only
```
.\browser.exe --cookies
```


# How to build? 
1. install gcc and golang 
2. git clone the repo
3. go mod init 
4. go mod tidy
5. go env -w CGO_ENABLED=1
6. go env -w GOOS=windows
7. go build -ldflags "-s -w" browser.go

binary will be in current directory

