package system

/* this file provides a mechanism for clients to connect to bus servers. */

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
)

// returns a new BusConnection.
func BusConnect(path string, handler busHandler) (conn *BusConnection, err error) {

	// check if file exists. if not, bail.
	_, err = os.Lstat(path)
	if err != nil {
		return
	}

	// resolve the address.
	addr, err := net.ResolveUnixAddr("unix", path)
	if err != nil {
		return
	}

	// connect.
	unixConn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return
	}

	// create the connection.
	conn = &BusConnection{
		path:        path,
		socket:      unixConn,
		incoming:    bufio.NewReader(unixConn),
		outgoing:    bufio.NewWriter(unixConn),
		busHandler:  handler,
		readHandler: jsonDataHandler,
	}

	return
}

// send a JSON event.
func (conn *BusConnection) Send(command string, params map[string]interface{}) bool {
	if params == nil {
		params = make(map[string]interface{})
	}
	b, err := json.Marshal([]interface{}{command, params})
	if err != nil {
		return false
	}
	b = append(b, '\n')
	if _, err = conn.socket.Write(b); err != nil {
		return false
	}
	return true
}

// read data from a connection continuously.
func (conn *BusConnection) Run() {
	for {

		// get the next line
		line, _, err := conn.incoming.ReadLine()

		// if there's an error, we will just continue and ignore it.
		if err != nil {
			return
		}

		// handle the event.
		conn.readHandler(conn, line)

	}
}

// handle a JSON event. returns true on success.
func jsonDataHandler(conn *BusConnection, data []byte) bool {

	// parse the data into i.
	var i interface{}
	err := json.Unmarshal(data, &i)

	// parse error!
	if err != nil {
		return false
	}

	// should be an array of format [PID, command, parameters].
	// PID        : type int
	// command    : type string
	// parameters : type map[string]interface{}
	c := i.([]interface{})

	var (
		source  *ClientProcess
		command string
		params  map[string]interface{}
	)

	// extract source.
	switch c[0].(type) {
	case int:
		source = newClientProcess(c[0].(int))
	default:
		return false
		// invalid
	}

	// extract command.
	switch c[1].(type) {
	case string:
		command = c[1].(string)
	default:
		return false
		// invalid.
	}

	// extract params.
	switch c[2].(type) {
	case map[string]interface{}:
		params = c[2].(map[string]interface{})
	default:
		return false
		// invalid.
	}

	// JSON data included incorrect types.
	if len(command) == 0 || params == nil {
		return false
	}

	// if a handler for this command exists, run it.
	conn.busHandler(source, command, params)

	return true
}
