package ui

// ChannelManager padroniza operações de canais e mensagens para todas as UIs
// Permite reuso de handlers e facilita testes
//
type ChannelManager interface {
	AddChannel(channel string)
	RemoveChannel(channel string)
	SetChannels(channels []string)
	SetActiveChannel(channel string)
	GetActiveChannel() string
	GetChannelList() []string
	AddMessageToChannel(channel, msg string)
	AddMessage(msg string)
}
