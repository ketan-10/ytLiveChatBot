package ytbot

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type LiveChatBotInput struct {
	Urls         []string
	RefetchCache bool
}

type LiveChatBot struct {
	ChatReaders map[string]<-chan *youtube.LiveChatMessage
	ChatWriters map[string]chan<- string
}

func NewLiveChatBot(input *LiveChatBotInput) *LiveChatBot {

	client := getClient(input.RefetchCache, youtube.YoutubeScope) // youtube.YoutubeReadonlyScope for readonly access
	service, err := youtube.NewService(context.Background(), option.WithHTTPClient(client))

	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}

	liveChatIds := fetchChatIds(input.Urls, service)

	chatReaders := make(map[string]<-chan *youtube.LiveChatMessage)
	chatWriters := make(map[string]chan<- string)
	for url, chatId := range liveChatIds {
		chatReaders[url] = readChat(service, chatId)
		chatWriters[url] = writeChat(service, chatId)
	}

	return &LiveChatBot{
		ChatReaders: chatReaders,
		ChatWriters: chatWriters,
	}
}

func readChat(service *youtube.Service, chatId string) <-chan *youtube.LiveChatMessage {

	messageChannel := make(chan *youtube.LiveChatMessage)
	const extraPercent float64 = 0.1

	// https://levelup.gitconnected.com/use-go-channels-as-promises-and-async-await-ee62d93078ec
	go func(chatId string) {
		defer close(messageChannel)

		messageContainer := make(map[string]bool)

		for {
			// get live chats from chatId
			call := service.LiveChatMessages.List(chatId, []string{"snippet", "authorDetails"})
			response, err := call.Do()
			if err != nil {
				fmt.Println("Closing Channel: ", chatId, " Error getting live chat messages:", err)
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
			go func(newMessage string) {
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
					fmt.Println("Error sending message: ", newMessage, " On Channel: ", chatId, " Error Was: ", err)
				}
			}(newMessage)
		}
	}(chatId)

	return messageChannel
}

func fetchChatIds(urls []string, service *youtube.Service) map[string]string {

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
	chatIds := make(map[string]string)
	for _, url := range urls {
		// chatIds = append(chatIds, <-responseChannel)
		chatIds[url] = <-responseChannel
	}

	return chatIds
}
