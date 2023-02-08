package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type Event struct {
	// The event type.
	Type string `json:"type"`

	// The event data.
	Payload json.RawMessage `json:"payload"`
}

type EventHandler func(event Event, client *Client) error

const (
	// EventJoinChannel is the event type for joining a channel.
	EventJoinChannel = "join_channel"

	// EventLeaveChannel is the event type for leaving a channel.
	EventLeaveChannel = "leave_channel"

	// EventSendMessage is the event type for sending a message.
	EventSendMessage = "send_message"

	// EventNewMessage is the event type for receiving a message.
	EventNewMessage = "new_message"

	// EventChangeChannel is the event type for changing channels.
	EventChangeChannel = "change_channel"
)

type SendMessageEvent struct {
	// Message is the message to send.
	Message string `json:"message"`

	// From is the sender of the message.
	From string `json:"from"`
}

type NewMessageEvent struct {
	SendMessageEvent
	Sent time.Time `json:"sent"`
}

func SendMessageHandler(event Event, c *Client) error {
	// Marshal the event payload into a SendMessageEvent.
	var sendMessageEvent SendMessageEvent
	if err := json.Unmarshal(event.Payload, &sendMessageEvent); err != nil {
		return fmt.Errorf("error unmarshaling bad payload event: %w", err)
	}

	// Prepare an outgoing message.
	var broadcastMessage NewMessageEvent
	broadcastMessage.Message = sendMessageEvent.Message
	broadcastMessage.From = sendMessageEvent.From
	broadcastMessage.Sent = time.Now()

	data, err := json.Marshal(broadcastMessage)
	if err != nil {
		return fmt.Errorf("error marshaling broadcast message: %w", err)
	}

	// Put the payload to the event
	var outgoingEvent Event
	outgoingEvent.Type = EventNewMessage
	outgoingEvent.Payload = data

	// Broadcast the message to all clients in the channel.
	for client := range c.manager.clients {
		if client.channel == c.channel {
			client.egress <- outgoingEvent
		}
	}
	
	return nil
}

type ChangeChannelEvent struct {
	// Channel is the channel to change to.
	Channel string `json:"channel"`
}

func ChangeChannelHandler(event Event, c *Client) error {
	// Marshal the event payload into a ChangeChannelEvent.
	var changeChannelEvent ChangeChannelEvent
	if err := json.Unmarshal(event.Payload, &changeChannelEvent); err != nil {
		return fmt.Errorf("error unmarshaling bad payload event: %w", err)
	}

	// Leave the current channel.
	// c.manager.LeaveChannel(c, c.channel)

	// Join the new channel.
	// c.manager.JoinChannel(c, changeChannelEvent.Channel)

	// Add client to the new channel.
	c.channel = changeChannelEvent.Channel

	return nil
}