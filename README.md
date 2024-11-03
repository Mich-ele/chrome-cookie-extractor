# Chrome Cookie Extractor

This tool bypasses Chrome v20+ app-bound encryption to extract cookies from the selected browser. It leverages Chrome's Remote Debugging capabilities, allowing users to securely retrieve and save their cookies for further use.
<br>
No admin rights required.

 <img src="https://i.imgur.com/yvxmBDv.png" alt="tool" width="550"/>
 
## Features
- Automatically locates browsers paths and user data directories.
- Connects to a running instance of Chrome with remote debugging enabled.
- Retrieves all cookies from the browser's session.
- Exports cookies in Netscape format into `cookies.txt`.
  
## Usage
Install dependencies
```console
go get github.com/gorilla/websocket
```
```console
go get golang.org/x/sys/windows/registry
```
Run
```console
go run main.go
```

## Credits
- [thewh1teagle](https://github.com/thewh1teagle) for python PoC
