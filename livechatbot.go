package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type LiveChatBotInput struct {
	urls         []string
	refetchCache bool
}

type LiveChatBot struct {
	liveChatIds []string
	chatReaders map[string]<-chan *youtube.LiveChatMessage
	chatWriters map[string]chan<- string
}

func NewLiveChatBot(input *LiveChatBotInput) *LiveChatBot {

	client := getClient(input.refetchCache, youtube.YoutubeReadonlyScope, youtube.YoutubeScope, youtube.YoutubeForceSslScope)
	service, err := youtube.NewService(context.Background(), option.WithHTTPClient(client))

	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}

	liveChatIds := fetchChatIds(input.urls, service)

	chatReaders := make(map[string]<-chan *youtube.LiveChatMessage)
	chatWriters := make(map[string]chan<- string)
	for _, chatId := range liveChatIds {
		chatReaders[chatId] = readChat(service, chatId)
		chatWriters[chatId] = writeChat(service, chatId)
	}

	return &LiveChatBot{
		liveChatIds: liveChatIds,
		chatReaders: chatReaders,
		chatWriters: chatWriters,
	}
}

func readChat(service *youtube.Service, chatId string) <-chan *youtube.LiveChatMessage {

	messageChannel := make(chan *youtube.LiveChatMessage)
	const extraPercent float64 = 0.5

	// https://levelup.gitconnected.com/use-go-channels-as-promises-and-async-await-ee62d93078ec
	go func(chatId string) {
		defer close(messageChannel)

		messageContainer := make(map[string]bool)

		for {
			// get live chats from chatId
			call := service.LiveChatMessages.List(chatId, []string{"snippet"})
			response, err := call.Do()
			if err != nil {
				break
			}

			for _, item := range response.Items {
				if val := messageContainer[item.Id]; !val {
					messageContainer[item.Id] = true
					messageChannel <- item
				}
			}

			time.Sleep(time.Millisecond * time.Duration((float64(response.PollingIntervalMillis) * (1 + extraPercent))))
		}
	}(chatId)
	return messageChannel

}

func writeChat(service *youtube.Service, chatId string) chan<- string {
	messageChannel := make(chan string)

	go func(chatId string) {
		for newMessage := range messageChannel {
			call := service.LiveChatMessages.Insert([]string{"snippet"}, &youtube.LiveChatMessage{
				Snippet: &youtube.LiveChatMessageSnippet{
					LiveChatId: chatId,
					Type:       "textMessageEvent",
					TextMessageDetails: &youtube.LiveChatTextMessageDetails{
						MessageText: newMessage,
					},
				},
			})
			_, err := call.Do()
			if err != nil {
				break
			}
		}
	}(chatId)

	return messageChannel
}

func fetchChatIds(urls []string, service *youtube.Service) []string {

	responseChannel := make(chan string)
	defer close(responseChannel)

	// parallelize the requests
	for _, url := range urls {
		go func(url string) {
			// get video-id from url
			vidRes, vidErr := service.Search.List([]string{"id"}).Q(url).Do()
			if vidErr != nil {
				log.Fatalf("Error getting video id: %v", vidErr)
			}
			// get live chat-id from video-id
			cidRes, cidErr := service.Videos.List([]string{"liveStreamingDetails"}).Id(vidRes.Items[0].Id.VideoId).Do()
			if cidErr != nil {
				log.Fatalf("Error getting live chat id: %v", cidErr)
			}
			responseChannel <- cidRes.Items[0].LiveStreamingDetails.ActiveLiveChatId
		}(url)
	}

	// wait for all responses
	var chatIds []string
	for range urls {
		chatIds = append(chatIds, <-responseChannel)
	}

	return chatIds
}
