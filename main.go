package main

import (
	"fmt"
	"log"
	"sync"
	"time"
	"os"
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

var wg = sync.WaitGroup{}

func getLattestMessageChannel(service *youtube.Service, chatId string, messageChannel chan<- string) {

	const extraPercent float64 = 0.5

	messageContainer := make(map[string]bool)
	defer close(messageChannel)

	for {
		response, err := getLiveChatsFromChatId(service, chatId)
		if err != nil {
			break
		}

		for _, item := range response.Items {
			if val := messageContainer[item.Id]; !val {
				messageContainer[item.Id] = true
				messageChannel <- item.Snippet.DisplayMessage
			}
		}

		time.Sleep(time.Millisecond * time.Duration((float64(response.PollingIntervalMillis) * (1 + extraPercent))))
	}

	wg.Done()
}

func main() {

	isNew := false;
	if len(os.Args) > 1  && os.Args[1] == "new"{ 
		  isNew = true;
	}

	client := getClient(youtube.YoutubeReadonlyScope, isNew)
	service, err := youtube.New(client)

	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}

	videoId, _ := getVideoIdFromUrl(service, "https://youtu.be/5qap5aO4i9A")

	liveChatId, _ := getLiveChatIdFromVideoId(service, videoId)

	messageChannel := make(chan string)

	wg.Add(1)
	go getLattestMessageChannel(service, liveChatId, messageChannel)

	for i := range messageChannel {
		fmt.Println(i)
	}
	wg.Wait()
}
