package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/sys/windows/registry"
)

var browserPaths = map[string][]string{
	"chrome": {
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Google\Chrome\Application\chrome.exe`),
	},
	"vivaldi": {
		`C:\Program Files\Vivaldi\Application\vivaldi.exe`,
		`C:\Program Files (x86)\Vivaldi\Application\vivaldi.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Vivaldi\Application\vivaldi.exe`),
	},
	"edge": {
		filepath.Join(os.Getenv("ProgramFiles(x86)"), `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(os.Getenv("ProgramFiles"), `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge\Application\msedge.exe`),
	},
	"opera": {
		`C:\Program Files\Opera\launcher.exe`,
		`C:\Program Files (x86)\Opera\launcher.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Programs\Opera\launcher.exe`),
	},
	"operagx": {
		`C:\Program Files\Opera GX\launcher.exe`,
		`C:\Program Files (x86)\Opera GX\launcher.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Programs\Opera GX\launcher.exe`),
	},
	"brave": {
		`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
		`C:\Program Files (x86)\BraveSoftware\Brave-Browser\Application\brave.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `BraveSoftware\Brave-Browser\Application\brave.exe`),
	},
}

var userDataDirs = map[string]string{
	"chrome":  filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "User Data"),
	"vivaldi": filepath.Join(os.Getenv("LOCALAPPDATA"), "Vivaldi", "User Data"),
	"edge":    filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "User Data"),
	"opera":   filepath.Join(os.Getenv("LOCALAPPDATA"), "Opera Software", "Opera Stable"),
	"operagx": filepath.Join(os.Getenv("LOCALAPPDATA"), "Opera Software", "Opera GX Stable"),
	"brave":   filepath.Join(os.Getenv("LOCALAPPDATA"), "BraveSoftware", "Brave-Browser", "User Data"),
}

var registryPaths = map[string]string{
	"chrome":  `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe`,
	"vivaldi": `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\vivaldi.exe`,
	"edge":    `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`,
	"opera":   `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\opera.exe`,
	"operagx": `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\opera_gx.exe`,
	"brave":   `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\brave.exe`,
}

const (
	DEBUG_PORT = 9222
	DEBUG_URL  = "http://localhost:9222/json"
)

type Cookie struct {
	Domain   string  `json:"domain"`
	HTTPOnly bool    `json:"httpOnly"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	Expiry   float64 `json:"expires"`
	Name     string  `json:"name"`
	Value    string  `json:"value"`
}

func findBrowserPath(browserName string) (string, error) {
	if paths, ok := browserPaths[browserName]; ok {
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("not found")
}

func getUserDataDir(browserName string) (string, error) {
	if dir, ok := userDataDirs[browserName]; ok {
		return dir, nil
	}
	return "", fmt.Errorf("not found")
}

func findBrowserPathInRegistry(browserName string) (string, error) {
	if regPath, ok := registryPaths[browserName]; ok {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, regPath, registry.QUERY_VALUE)
		if err != nil {
			return "", err
		}
		defer key.Close()

		path, _, err := key.GetStringValue("")
		return path, err
	}
	return "", fmt.Errorf("not found")
}

func getDebugWsURL() (string, error) {
	resp, err := http.Get(DEBUG_URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data) > 0 {
		if wsURL, ok := data[0]["webSocketDebuggerUrl"].(string); ok {
			return strings.TrimSpace(wsURL), nil
		}
	}
	return "", fmt.Errorf("webSocketDebuggerUrl not found")
}

func killBrowser(browserName string) error {
	processName := map[string]string{
		"chrome":  "chrome.exe",
		"vivaldi": "vivaldi.exe",
		"edge":    "msedge.exe",
		"opera":   "opera.exe",
		"operagx": "opera_gx.exe",
		"brave":   "brave.exe",
	}
	if name, ok := processName[browserName]; ok {
		return exec.Command("taskkill", "/F", "/IM", name).Run()
	}
	return fmt.Errorf("not found")
}

func startDebuggedChrome(chromePath, userDataDir string) {
	cmd := exec.Command(chromePath, fmt.Sprintf("--remote-debugging-port=%d", DEBUG_PORT), "--remote-allow-origins=*", "--headless", fmt.Sprintf("--user-data-dir=%s", userDataDir))
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Start()
	time.Sleep(2 * time.Second)
}

func jsonToNetscape(jsonCookies string) string {
	var cookies []Cookie
	json.Unmarshal([]byte(jsonCookies), &cookies)

	var netscapeCookies []string
	for _, cookie := range cookies {
		httpOnly := "FALSE"
		if cookie.HTTPOnly {
			httpOnly = "TRUE"
		}
		secure := "FALSE"
		if cookie.Secure {
			secure = "TRUE"
		}
		netscapeCookies = append(netscapeCookies, fmt.Sprintf("%s\t%s\t%s\t%s\t%.0f\t%s\t%s", cookie.Domain, httpOnly, cookie.Path, secure, cookie.Expiry, cookie.Name, cookie.Value))
	}

	return strings.Join(netscapeCookies, "\n")
}

func saveCookiesToFile(filename, cookies string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(cookies)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	var input int
	fmt.Print("[1] Chrome\n[2] Vivaldi\n[3] Edge\n[4] Opera\n[5] Opera GX\n[6] Brave\n\nEnter number: ")

	_, err := fmt.Scan(&input)
	if err != nil {
		fmt.Println("Error reading input:", err)
		return
	}

	browsers := map[int]string{
		1: "chrome",
		2: "vivaldi",
		3: "edge",
		4: "opera",
		5: "operagx",
		6: "brave",
	}

	if browserName, ok := browsers[input]; ok {
		chromePath, err := findBrowserPathInRegistry(browserName)
		if err != nil || chromePath == "" {
			chromePath, err = findBrowserPath(browserName)
			if err != nil || chromePath == "" {
				return
			}
		}

		userDataDir, err := getUserDataDir(browserName)
		if err != nil {
			log.Fatalf("Error getting user data directory: %v", err)
		}

		killBrowser(browserName)
		startDebuggedChrome(chromePath, userDataDir)

		url, err := getDebugWsURL()
		if err != nil {
			log.Fatalf("Error getting WebSocket URL: %v", err)
		}

		ws, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Fatalf("Error connecting WebSocket: %v", err)
		}
		defer ws.Close()

		message := map[string]interface{}{
			"id":     1,
			"method": "Network.getAllCookies",
		}
		messageJSON, _ := json.Marshal(message)
		if err := ws.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
			log.Fatalf("Error sending WebSocket message: %v", err)
		}

		_, response, err := ws.ReadMessage()
		if err != nil {
			log.Fatalf("Error reading WebSocket message: %v", err)
		}

		var result map[string]interface{}
		json.Unmarshal(response, &result)
		cookies := result["result"].(map[string]interface{})["cookies"]
		cookieData, _ := json.MarshalIndent(cookies, "", "  ")
		netscapeCookies := jsonToNetscape(string(cookieData))

		err = saveCookiesToFile("cookies.txt", netscapeCookies)
		if err != nil {
			log.Fatalf("Error exporting cookies: %v", err)
		} else {
			fmt.Printf("[!] Succesfully exported %d cookies\n", len(strings.Split(netscapeCookies, "\n")))
		}

		killBrowser(browserName)
	} else {
		main()
	}
}
