package wgbind

import "sync"

var packetBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, maxWGPacket)
	},
}

func getPacketBuf() []byte {
	return packetBufPool.Get().([]byte)
}

func putPacketBuf(buf []byte) {
	if buf == nil || len(buf) != maxWGPacket {
		return
	}
	packetBufPool.Put(buf)
}
