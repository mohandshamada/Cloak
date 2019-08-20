// This is base on https://github.com/golang/go/blob/0436b162397018c45068b47ca1b5924a3eafdee0/src/net/net_fake.go#L173

package multiplex

import (
	"io"
	"sync"
)

const DATAGRAM_NUMBER_LIMIT = 1024

// datagramBuffer is the same as bufferedPipe with the exception that it's message-oriented,
// instead of byte-oriented. The integrity of datagrams written into this buffer is preserved.
// it won't get chopped up into individual bytes
type datagramBuffer struct {
	buf    [][]byte
	closed bool
	rwCond *sync.Cond
}

func NewDatagramBuffer() *datagramBuffer {
	d := &datagramBuffer{
		buf:    make([][]byte, 0),
		rwCond: sync.NewCond(&sync.Mutex{}),
	}
	return d
}

func (d *datagramBuffer) Read(target []byte) (int, error) {
	d.rwCond.L.Lock()
	defer d.rwCond.L.Unlock()
	for {
		if d.closed && len(d.buf) == 0 {
			return 0, io.EOF
		}

		if len(d.buf) > 0 {
			break
		}
		d.rwCond.Wait()
	}
	// TODO: return error if len(target) is smaller than the datagram
	var data []byte
	data, d.buf = d.buf[0], d.buf[1:]
	copy(target, data)
	// err will always be nil because we have already verified that buf.Len() != 0
	d.rwCond.Broadcast()
	return len(data), nil
}

func (d *datagramBuffer) Write(input []byte) (int, error) {
	d.rwCond.L.Lock()
	defer d.rwCond.L.Unlock()
	for {
		if d.closed {
			return 0, io.ErrClosedPipe
		}
		if len(d.buf) <= DATAGRAM_NUMBER_LIMIT {
			// if d.buf gets too large, write() will panic. We don't want this to happen
			break
		}
		d.rwCond.Wait()
	}
	data := make([]byte, len(input))
	copy(data, input)
	d.buf = append(d.buf, data)
	// err will always be nil
	d.rwCond.Broadcast()
	return len(data), nil
}

func (d *datagramBuffer) Close() error {
	d.rwCond.L.Lock()
	defer d.rwCond.L.Unlock()

	d.closed = true
	d.rwCond.Broadcast()
	return nil
}

func (d *datagramBuffer) Len() int {
	d.rwCond.L.Lock()
	defer d.rwCond.L.Unlock()
	return len(d.buf)
}