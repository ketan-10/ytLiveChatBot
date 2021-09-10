# ytLiveChatBot

ytLiveChatBot is a golang library for cli base applications that makes it easy to read and write live-chat on/from youtube live streams using youtube api.

It is designed for handling multiple live-streams at a time.
But can also be used for a single live-stream


## Features
- Abstracts youtube API Oauth logic 
- converts youtube live-stream chat in easily manageable readable and writable go channels
- `ChatReader` channel will receive incoming chat-messages
- Anything written on `ChatWriter` channel will be   

## Setup

- **Obtain OAuth 2.0 credentials from the Google API Console**       
    Go through [before you start section](https://developers.google.com/youtube/v3/getting-started#before-you-start) of youtube api, and download `client_secret.json`
    
![image](https://user-images.githubusercontent.com/35309821/132854373-bb1d58e9-36dc-46a9-b61d-9b52ac4b39a5.png)

- **Make sure `client_secret.json` is stored inside your application root directory.**

## Usage

```go
import "github.com/ketan-10/ytLiveChatBot"
```

- Intantiate `LiveChatBot` with youtube live-stream Urls, urls can be in any of the following formats `https://www.youtube.com/watch?v=5qap5aO4i9A", "5qap5aO4i9A", "https://youtu.be/5qap5aO4i9A`
```go
var chatBot *ytbot.LiveChatBot = ytbot.NewLiveChatBot(&ytbot.LiveChatBotInput{
  Urls: []string{"https://youtu.be/KSU023LJKS", "https://youtu.be/eo22kjof2", "5qap5aO4i9A"},
})
```

- `LiveChatBot` have `ChatReaders` and `ChatWriters` which are Maps having `key` of youtube-url and value as respective chat reader or writer channel

```go
var chatReaders map[string]<-chan *youtube.LiveChatMessage = chatBot.ChatReaders
var chatWriters map[string]chan<- string = chatBot.ChatWriters
```

- `NewLiveChatBot` func will initiate oauth flow, it will print the google oauth url in console where user has open the url in browser and grant youtube Access.  

Choose Account             |  Grant Access
:-------------------------:|:-------------------------:
![image](https://user-images.githubusercontent.com/35309821/132854091-e5652e21-a552-43c2-9f7d-42ef69d2cf2e.png) | ![image](https://user-images.githubusercontent.com/35309821/132854253-6ed4efc1-b7b4-4c76-b1b8-7a5cc35d1975.png)

- **Google Sign in** will show a token, which to be copy pasted to console 

- For subsequent re-runs token will be cached in `.credentials` file in home directory, to ignore cache use `RefetchCache: true`
```go
chatBot := ytbot.NewLiveChatBot(&ytbot.LiveChatBotInput{
  RefetchCache: true,
  Urls:         []string{"5qap5aO4i9A", "3304aO430C"},
},
```

#### Examples
1) Read from console and write directly to youtube live-stream chat
```go
package main

import (
	"bufio"
	"os"
	"sync"

	"github.com/ketan-10/ytLiveChatBot"
)

var wg = sync.WaitGroup{}

func main() {
	chatBot := ytbot.NewLiveChatBot(&ytbot.LiveChatBotInput{
		Urls: []string{"5qap5aO4i9A"},
	})
	chatWriter := chatBot.ChatWriters["5qap5aO4i9A"]

	scanner := bufio.NewScanner(os.Stdin)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if scanner.Scan() {
				newChat := scanner.Text()
				chatWriter <- newChat
			}
		}
	}()

	wg.Wait()
}

```
2) Read multiple youtube live-stream chat and print live result.
```go
package main

import (
	"fmt"
	"sync"

	"github.com/ketan-10/ytLiveChatBot"
	"google.golang.org/api/youtube/v3"
)

var wg = sync.WaitGroup{}
// read multiple youtube live stream chat
func main() {

	chatBot := ytbot.NewLiveChatBot(&ytbot.LiveChatBotInput{
		// enter list of livestream urls
		Urls: []string{"https://youtu.be/KSU023LJKS", "https://youtu.be/eo22kjof2", "5qap5aO4i9A"},
	})
	for url, valueReader := range chatBot.ChatReaders {
		wg.Add(1)
		go readChannel(valueReader, url)
	}
	wg.Wait()
}

func readChannel(valueReader <-chan *youtube.LiveChatMessage, url string) {
	defer wg.Done()
	for item := range valueReader {
		newMessage := item.Snippet.DisplayMessage
		fmt.Println("Form stream: " + url + " -> Message: " + newMessage)
	}
}

```

3) If chat-message have prefix `/a ` send the message to all other live-streams
```go
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/ketan-10/ytLiveChatBot"
	"google.golang.org/api/youtube/v3"
)

func main() {

	refetchCache := false
	if len(os.Args) > 1 && os.Args[1] == "ignoreCache" {
		refetchCache = true
	}

	chatBot := ytbot.NewLiveChatBot(&ytbot.LiveChatBotInput{
		RefetchCache: refetchCache,
		Urls:         []string{"5qap5aO4i9A", "WUG1AocK2Ys"},
	})

	var wg = sync.WaitGroup{}

	const prefix string = "/a "
	re := regexp.MustCompile("^(" + prefix + ")*")

	fmt.Println("Starting bot...")
	for rUrl, readChannel := range chatBot.ChatReaders {
		wg.Add(1)
		go func(readChannel <-chan *youtube.LiveChatMessage, rUrl string) {
			defer wg.Done()
			for item := range readChannel {
				newMessage := item.Snippet.DisplayMessage
				if !strings.HasPrefix(newMessage, prefix) {
					continue
				}
				// remove prefix
				newMessage = re.ReplaceAllString(newMessage, "")

				fmt.Println(item.AuthorDetails)
				// send to all other live-stream
				sendMessage := "@" + item.AuthorDetails.DisplayName + " From: " + rUrl + " Said: " + newMessage
				fmt.Println(sendMessage)
				for wUrl, writeChannel := range chatBot.ChatWriters {
					if wUrl != rUrl {
						writeChannel <- sendMessage
					}
				}
			}

		}(readChannel, rUrl)
	}

	wg.Wait()
}

```


### References
[GO](https://www.youtube.com/watch?v=YS4e4q9oBaU)

[Are there any security concerns with sharing the client secrets of a Google API project?](https://stackoverflow.com/questions/62315535/are-there-any-security-concerns-with-sharing-the-client-secrets-of-a-google-api)

[Youtube API examples](https://developers.google.com/youtube/v3/code_samples/go)

[Google OAuth2](https://developers.google.com/identity/protocols/oauth2/native-app)

[Flutter Example Video 1](https://www.youtube.com/watch?v=3KfclTlg51c)&nbsp;&nbsp;&nbsp;[Flutter Example Video 2](https://www.youtube.com/watch?v=nUOoqSOJId0)

[Youtube API path](https://stackoverflow.com/questions/57263074/cant-get-live-chat-from-stream-i-do-not-own)

[Why need Authorization-code](https://stackoverflow.com/questions/16321455/what-is-the-difference-between-the-oauth-authorization-code-and-implicit-workflo)

[Publish go module](https://go.dev/blog/publishing-go-modules)

`go run . new`

`go mod init github.com/ketan-10/combine-youtube-live-chat`
`go mod tidy`



