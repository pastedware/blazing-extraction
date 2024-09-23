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
> install gcc and golang 
> git clone the repo
> go mod init 
> go mod tidy
> go env -w CGO_ENABLED=1
> go env -w GOOS=windows
> go build -ldflags "-s -w" browser.go
> binary will be in current directory

# For the Nerds

This extraction tool makes full use of goroutines and channels to achieve high parallelism.

## Parallelism Structure

### 1. Top-level parallelism

The program splits into two main concurrent paths:
- One for default browsers
- One for Opera browsers

These run in parallel.

### 2. Browser-level parallelism

For each browser type (Chrome, Edge, Brave, etc.), the program creates separate goroutines for:
- Password decryption
- Cookie decryption

These run in parallel for each browser.

### 3. Entry-level parallelism

For each password and cookie entry within a browser, a new goroutine is spawned. This is where the parallelism can become very high.

> **Note**: Future improvement could involve creating a customizable pool instead of spawning a goroutine for each entry.

## Parallelism Diagram

```mermaid
graph TD
    A[main] --> B[decryptBrowserData]
    B --> C[defaultbrowser]
    B --> D[operaBrowsers]
    
    C --> E1[decryptlogin: Chrome]
    C --> F1[decryptcookie: Chrome]
    
    E1 --> G1[goroutines for Chrome passwords]

    F1 --> H1[goroutines for Chrome cookies]

    
    D --> I[decryptlogin: Opera Stable]
    D --> J[decryptcookie: Opera Stable]
    
    I --> K1[goroutines for Opera password]

    J --> L1[goroutines for Opera cookie]
``` 
## Example Usage



## Limitations
Due to its very high parallism usage, it might consume a lot of resources if the target has many entries, especially when decrypting cookies.
(if you just want the passwords, just use --passwords.)
