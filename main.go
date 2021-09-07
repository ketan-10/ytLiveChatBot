package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func getVideoIdFromUrl(service *youtube.Service, url string) (string, error) {
	call := service.Search.List([]string{"id"})
	// call := service.Channels.List([]string{part})
	call = call.Q(url)
	response, err := call.Do()
	if err != nil {
		return "", err
	}
	return response.Items[0].Id.VideoId, nil
}

func getLiveChatIdFromVideoId(service *youtube.Service, videoId string) (string, error) {
	call := service.Videos.List([]string{"liveStreamingDetails"})

	call = call.Id(videoId)
	response, err := call.Do()
	if err != nil {
		return "", err
	}
	return response.Items[0].LiveStreamingDetails.ActiveLiveChatId, nil
}

func getLiveChatsFromChatId(service *youtube.Service, chatId string) (*youtube.LiveChatMessageListResponse, error) {
	call := service.LiveChatMessages.List(chatId, []string{"snippet"})
	response, err := call.Do()
	if err != nil {
		return nil, err
	}
	return response, nil
}

func getLattestMessageChannel(service *youtube.Service, chatId string) <-chan string {

	messageChannel := make(chan string)
	const extraPercent float64 = 0.5

	// https://levelup.gitconnected.com/use-go-channels-as-promises-and-async-await-ee62d93078ec
	go func(chatId string) {
		defer close(messageChannel)

		messageContainer := make(map[string]bool)

		for {
			response, err := getLiveChatsFromChatId(service, chatId)
			if err != nil {
				break
			}

			for _, item := range response.Items {
				if val := messageContainer[item.Id]; !val {
					messageContainer[item.Id] = true
					messageChannel <- item.Snippet.DisplayMessage
					// messageChannel <- item.Snippet.TextMessageDetails.MessageText
				}
			}

			time.Sleep(time.Millisecond * time.Duration((float64(response.PollingIntervalMillis) * (1 + extraPercent))))
		}
	}(chatId)
	return messageChannel

}

func getChatIdFromUrl(url string, service *youtube.Service) string {
	videoId, _ := getVideoIdFromUrl(service, url)

	liveChatId, _ := getLiveChatIdFromVideoId(service, videoId)

	return liveChatId

}

func readAll(urls []string, service *youtube.Service) []string {

	responseChannel := make(chan string)
	defer close(responseChannel)

	for _, url := range urls {
		go func(url string) {
			chatId := getChatIdFromUrl(url, service)
			responseChannel <- chatId
		}(url)
	}

	var chatIds []string
	for range urls {
		chatIds = append(chatIds, <-responseChannel)
	}

	return chatIds
}

// writers

func sendLiveChatFromChatId(service *youtube.Service, chatId string, message string) (*youtube.LiveChatMessage, error) {
	call := service.LiveChatMessages.Insert([]string{"snippet"}, &youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			LiveChatId: chatId,
			Type:       "textMessageEvent",
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: message,
			},
		},
	})
	response, err := call.Do()
	if err != nil {
		return nil, err
	}
	return response, nil
}

func sendMessageChannel(service *youtube.Service, chatId string) chan<- string {
	messageChannel := make(chan string)

	go func(chatId string) {
		for newChat := range messageChannel {
			sendLiveChatFromChatId(service, chatId, newChat)
		}
	}(chatId)

	return messageChannel
}

func main() {

	isNew := false
	if len(os.Args) > 1 && os.Args[1] == "new" {
		isNew = true
	}

	client := getClient(isNew, youtube.YoutubeReadonlyScope, youtube.YoutubeScope, youtube.YoutubeForceSslScope)
	// client := getClient(, isNew)
	service, err := youtube.NewService(context.Background(), option.WithHTTPClient(client))

	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}

	// liveChatId := readable(, service)
	liveChatIds := readAll([]string{"https://youtu.be/5qap5aO4i9A"}, service)

	chatReaders := make(map[string]<-chan string)
	chatWriters := make(map[string]chan<- string)

	for _, chatId := range liveChatIds {
		chatReaders[chatId] = getLattestMessageChannel(service, chatId)
		chatWriters[chatId] = sendMessageChannel(service, chatId)
	}

	var wg = sync.WaitGroup{}
	const repeat string = "/r "
	for _, valueReader := range chatReaders {
		wg.Add(1)
		go func(valueReader <-chan string) {
			defer wg.Done()
			for newMessage := range valueReader {
				if !strings.HasPrefix(newMessage, repeat) {
					continue
				}
				fmt.Println(newMessage)
				re := regexp.MustCompile("^(" + repeat + ")*")
				newMessage = re.ReplaceAllString(newMessage, "")
				fmt.Println("after: ", newMessage)

				for _, valueWriter := range chatWriters {
					// if keyWriter != keyReader {
					valueWriter <- newMessage
					// }
				}
			}

		}(valueReader)
	}

	wg.Wait()
}
