package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

const (
	IrcMessageSize = 512
)

const (
	ModeWallops   = 1 << 2
	ModeInvisible = 1 << 3
)

type MessageHandlerKey int

type Message struct {

	// Either the servername or nick!user@host.
	Prefix string

	// Either the command name (e.g. "NICK) or the 3-digit code (e.g. 001).
	Command string
}

type MessageHandler func(*Message)

type Conn struct {
	conn net.Conn

	wg sync.WaitGroup

	callbacksLock sync.Mutex
	callbacks     map[string]map[MessageHandlerKey]MessageHandler

	shutdown chan struct{}
	write    chan string

	counter MessageHandlerKey

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
					log.Fatal("jkjkhkjh")
				}
			}
		}
	}
}

func main() {

	//fmt.Printf("%d", len([]byte(nil)))

	l := []int{0}

	fmt.Println(l[0])
	for _, n := range l[1:] {
		fmt.Println(n)
	}
	//conn, err := Dial("tcp", "localhost:6667")
	//if err != nil {
	//	log.Fatalf("error connecting: %v", err)
	//}
	//
	//conn.Nick("mana")
	//conn.User("michele", "Michele Phung")
	//
	//sc := bufio.NewScanner(os.Stdin)
	//for sc.Scan() {
	//	fmt.Print(sc.Text())
	//}
	//
	//fmt.Println("closing")
	//
	//conn.Close()
	//fmt.Println("Done")
}

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
	for i := 0; i < 2; i++ {
		c.shutdown <- struct{}{}
	}

	c.wg.Wait()

	log.Println("")
	fmt.Println("Finished shutdown.")
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

// Runs all callbacks registered for the specified code. Each callback is run
// in its own goroutine.
func (c *Conn) runCallbacks(code string, message *Message) {
	c.callbacksLock.Lock()
	defer c.callbacksLock.Unlock()

	cbs, ok := c.callbacks[code]
	if ok {
		for _, cb := range cbs {
			go cb(message)
		}
	}

	log.Println("Finished running callbacks.")
}

//func (c *Conn) runCallbacksNewGoroutine(code string) {
//
//}
