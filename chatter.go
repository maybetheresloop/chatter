package chatter

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

type MessageHandlerKey int

const (
	IrcMessageSize = 512
)

const (
	ModeWallops   = 1 << 2
	ModeInvisible = 1 << 3
)

type ChannelKey struct {
	channel string
	key     string
}

// Message represents an IRC message received from the remote connection.
type Message struct {

	// Either the servername or nick!user@host.
	Prefix string

	// Either the command name (e.g. "NICK") or the 3-digit code (e.g. 001).
	Command string
}

type MessageHandler func(*Message)

type Conn struct {
	conn          net.Conn
	wg            sync.WaitGroup
	callbacksLock sync.Mutex
	callbacks     map[string]map[MessageHandlerKey]MessageHandler
	shutdown      chan struct{}
	write         chan string

	errors  chan error
	counter MessageHandlerKey

	// Configuration for whether or not to spawn a new goroutine for each
	// callback.
	callbackNewGoroutine bool
}

func (c *Conn) readLoop() {
	defer c.wg.Done()

	r := bufio.NewReaderSize(c.conn, IrcMessageSize)
	for {

		line, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("why")
		}

		fmt.Print(line)
	}

}

func (c *Conn) writeLoop() {
	defer c.wg.Done()
	fmt.Println("in write loop")

	for {
		select {
		case <-c.shutdown:
			return
		case message, ok := <-c.write:
			if ok {
				_, err := c.conn.Write([]byte(message))
				if err != nil {
					log.Fatal("")
				}
			}
		}
	}
}

// Dial creates an IRC connection to the address on the named network.
func Dial(network string, address string) (*Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}

	c := &Conn{
		conn:     conn,
		write:    make(chan string, 10),
		shutdown: make(chan struct{}),
	}

	go c.readLoop()
	go c.writeLoop()
	c.wg.Add(2)

	return c, nil
}

// On registers a callback function for the specified code.
//
// Whenever a message identified by the specified code is received from the
// remote server, the callback function will be called in its own goroutine.
func (c *Conn) On(code string, cb MessageHandler) MessageHandlerKey {
	cbs, ok := c.callbacks[code]
	if !ok {
		cbs = make(map[MessageHandlerKey]MessageHandler)
		c.callbacks[code] = cbs
	}

	c.counter += 1
	cbs[c.counter] = cb
	return c.counter
}

// RemoveListeners removes the callback identified by the specified key from
// the callbacks registered for the specified code.
func (c *Conn) RemoveListener(code string, key MessageHandlerKey) {
	cbs, ok := c.callbacks[code]
	if ok {
		delete(cbs, key)
	}
}

func (c *Conn) RemoveAllListeners(code string) {

	cbs, ok := c.callbacks[code]
	if ok {
		for k := range cbs {
			delete(cbs, k)
		}
	}
}

func (c *Conn) Close() {
	log.Println("Shutting down all read/write loops.")

	for i := 0; i < 2; i++ {
		c.shutdown <- struct{}{}
	}

	c.wg.Wait()

	log.Println("Shut down all read/write loops.")
}

func (c *Conn) Nick(nickname string) {
	c.write <- fmt.Sprintf("NICK %s\r\n", nickname)
}

func (c *Conn) User(user string, realname string, mode ...int) {
	mask := 0
	if len(mode) > 0 {
		mask = mode[0]
	}
	c.write <- fmt.Sprintf("USER %s %d * :%s\r\n", user, mask, realname)
}

func (c *Conn) Privmsg(target string, text string) {
	c.write <- fmt.Sprintf("PRIVMSG %s :%s\r\n", target, text)
}

func (c *Conn) Motd(target ...string) {
	if len(target) > 0 {
		c.write <- fmt.Sprintf("MOTD %s\r\n", target[0])
	} else {
		c.write <- fmt.Sprintf("MOTD\r\n")
	}
}

func makeJoinMessage(channelKeys []ChannelKey) string {
	if len(channelKeys) == 0 {
		return fmt.Sprintf("JOIN 0\r\n")
	}

	channels := bytes.NewBufferString("JOIN ")
	var keys bytes.Buffer

	channels.WriteString(channelKeys[0].channel)
	keys.WriteString(channelKeys[0].key)

	for _, channel := range channelKeys[1:] {
		channels.WriteByte(byte(','))
		channels.WriteString(channel.channel)

		keys.WriteByte(byte(','))
		keys.WriteString(channel.key)
	}

	channels.WriteByte(byte(' '))
	channels.Write(keys.Bytes())

	// Write CRLF.
	channels.WriteByte(byte('\r'))
	channels.WriteByte(byte('\n'))

	return channels.String()
}

func (c *Conn) Join(channelKeys []ChannelKey) {
	c.write <- makeJoinMessage(channelKeys)
}

func (c *Conn) Part(channels []string, message ...string) {
	if len(channels) == 0 {
		return
	}
}

func (c *Conn) Ping(server1 string, server2 ...string) {
	if len(server2) > 0 {
		c.write <- fmt.Sprintf("PING %s %s\r\n", server1, server2[0])
	} else {
		c.write <- fmt.Sprintf("PING %s\r\n", server1)
	}
}

func (c *Conn) Away(text ...string) {
	if len(text) > 0 {
		c.write <- fmt.Sprintf("AWAY :%s\r\n", text[0])
	} else {
		c.write <- fmt.Sprintf("AWAY\r\n")
	}
}

func (c *Conn) ModeUser() {

}

//func (c *Conn) Lusers()

// Runs all callbacks registered for the specified code. Each callback is run
// in its own goroutine.
func (c *Conn) runCallbacks(code string, message *Message) {
	c.callbacksLock.Lock()
	defer c.callbacksLock.Unlock()

	log.Printf("Running callbacks for code: %s\n", code)

	cbs, ok := c.callbacks[code]
	if ok {
		for _, cb := range cbs {
			go cb(message)
		}
	}

	log.Printf("Finished running callbacks for code: %s\n", code)
}
