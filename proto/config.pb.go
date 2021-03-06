// Code generated by protoc-gen-gogo.
// source: cockroach/proto/config.proto
// DO NOT EDIT!

package proto

import proto1 "github.com/gogo/protobuf/proto"
import math "math"

// discarding unused import gogoproto "gogoproto/gogo.pb"

import io "io"
import fmt "fmt"
import github_com_gogo_protobuf_proto "github.com/gogo/protobuf/proto"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto1.Marshal
var _ = math.Inf

// Attributes specifies a list of arbitrary strings describing
// node topology, store type, and machine capabilities.
type Attributes struct {
	Attrs            []string `protobuf:"bytes,1,rep,name=attrs" json:"attrs" yaml:"attrs,flow"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *Attributes) Reset()         { *m = Attributes{} }
func (m *Attributes) String() string { return proto1.CompactTextString(m) }
func (*Attributes) ProtoMessage()    {}

func (m *Attributes) GetAttrs() []string {
	if m != nil {
		return m.Attrs
	}
	return nil
}

// Replica describes a replica location by node ID (corresponds to a
// host:port via lookup on gossip network), store ID (identifies the
// device) and associated attributes. Replicas are stored in Range
// lookup records (meta1, meta2).
type Replica struct {
	NodeID  NodeID  `protobuf:"varint,1,opt,name=node_id,customtype=NodeID" json:"node_id"`
	StoreID StoreID `protobuf:"varint,2,opt,name=store_id,customtype=StoreID" json:"store_id"`
	// Combination of node & store attributes.
	Attrs            Attributes `protobuf:"bytes,3,opt,name=attrs" json:"attrs"`
	XXX_unrecognized []byte     `json:"-"`
}

func (m *Replica) Reset()         { *m = Replica{} }
func (m *Replica) String() string { return proto1.CompactTextString(m) }
func (*Replica) ProtoMessage()    {}

func (m *Replica) GetAttrs() Attributes {
	if m != nil {
		return m.Attrs
	}
	return Attributes{}
}

// RangeDescriptor is the value stored in a range metadata key.
// A range is described using an inclusive start key, a non-inclusive end key,
// and a list of replicas where the range is stored.
type RangeDescriptor struct {
	RaftID int64 `protobuf:"varint,1,opt,name=raft_id" json:"raft_id"`
	// StartKey is the first key which may be contained by this range.
	StartKey Key `protobuf:"bytes,2,opt,name=start_key,customtype=Key" json:"start_key"`
	// EndKey marks the end of the range's possible keys.  EndKey itself is not
	// contained in this range - it will be contained in the immediately
	// subsequent range.
	EndKey Key `protobuf:"bytes,3,opt,name=end_key,customtype=Key" json:"end_key"`
	// Replicas is the set of replicas on which this range is stored, the
	// ordering being arbitrary and subject to permutation.
	Replicas         []Replica `protobuf:"bytes,4,rep,name=replicas" json:"replicas"`
	XXX_unrecognized []byte    `json:"-"`
}

func (m *RangeDescriptor) Reset()         { *m = RangeDescriptor{} }
func (m *RangeDescriptor) String() string { return proto1.CompactTextString(m) }
func (*RangeDescriptor) ProtoMessage()    {}

func (m *RangeDescriptor) GetRaftID() int64 {
	if m != nil {
		return m.RaftID
	}
	return 0
}

func (m *RangeDescriptor) GetReplicas() []Replica {
	if m != nil {
		return m.Replicas
	}
	return nil
}

// GCPolicy defines garbage collection policies which apply to MVCC
// values within a zone.
//
// TODO(spencer): flesh this out to include maximum number of values
//   as well as whether there's an intersection between max values
//   and TTL or a union.
type GCPolicy struct {
	// TTLSeconds specifies the maximum age of a value before it's
	// garbage collected. Only older versions of values are garbage
	// collected. Specifying <=0 mean older versions are never GC'd.
	TTLSeconds       int32  `protobuf:"varint,1,opt,name=ttl_seconds" json:"ttl_seconds"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *GCPolicy) Reset()         { *m = GCPolicy{} }
func (m *GCPolicy) String() string { return proto1.CompactTextString(m) }
func (*GCPolicy) ProtoMessage()    {}

func (m *GCPolicy) GetTTLSeconds() int32 {
	if m != nil {
		return m.TTLSeconds
	}
	return 0
}

// AcctConfig holds accounting configuration.
type AcctConfig struct {
	ClusterId        string `protobuf:"bytes,1,opt,name=cluster_id" json:"cluster_id" yaml:"cluster_id,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *AcctConfig) Reset()         { *m = AcctConfig{} }
func (m *AcctConfig) String() string { return proto1.CompactTextString(m) }
func (*AcctConfig) ProtoMessage()    {}

func (m *AcctConfig) GetClusterId() string {
	if m != nil {
		return m.ClusterId
	}
	return ""
}

// PermConfig holds permission configuration, specifying read/write ACLs.
type PermConfig struct {
	// ACL lists users with read permissions.
	Read []string `protobuf:"bytes,1,rep,name=read" json:"read" yaml:"read,omitempty"`
	// ACL lists users with write permissions.
	Write            []string `protobuf:"bytes,2,rep,name=write" json:"write" yaml:"write,omitempty"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *PermConfig) Reset()         { *m = PermConfig{} }
func (m *PermConfig) String() string { return proto1.CompactTextString(m) }
func (*PermConfig) ProtoMessage()    {}

func (m *PermConfig) GetRead() []string {
	if m != nil {
		return m.Read
	}
	return nil
}

func (m *PermConfig) GetWrite() []string {
	if m != nil {
		return m.Write
	}
	return nil
}

// ZoneConfig holds configuration that is needed for a range of KV pairs.
type ZoneConfig struct {
	// ReplicaAttrs is a slice of Attributes, each describing required attributes
	// for each replica in the zone. The order in which the attributes are stored
	// in ReplicaAttrs is arbitrary and may change.
	ReplicaAttrs  []Attributes `protobuf:"bytes,1,rep,name=replica_attrs" json:"replica_attrs" yaml:"replicas,omitempty"`
	RangeMinBytes int64        `protobuf:"varint,2,opt,name=range_min_bytes" json:"range_min_bytes" yaml:"range_min_bytes,omitempty"`
	RangeMaxBytes int64        `protobuf:"varint,3,opt,name=range_max_bytes" json:"range_max_bytes" yaml:"range_max_bytes,omitempty"`
	// If GC policy is not set, uses the next highest, non-null policy
	// in the zone config hierarchy, up to the default policy if necessary.
	GC               *GCPolicy `protobuf:"bytes,4,opt,name=gc" json:"gc,omitempty" yaml:"gc,omitempty"`
	XXX_unrecognized []byte    `json:"-"`
}

func (m *ZoneConfig) Reset()         { *m = ZoneConfig{} }
func (m *ZoneConfig) String() string { return proto1.CompactTextString(m) }
func (*ZoneConfig) ProtoMessage()    {}

func (m *ZoneConfig) GetReplicaAttrs() []Attributes {
	if m != nil {
		return m.ReplicaAttrs
	}
	return nil
}

func (m *ZoneConfig) GetRangeMinBytes() int64 {
	if m != nil {
		return m.RangeMinBytes
	}
	return 0
}

func (m *ZoneConfig) GetRangeMaxBytes() int64 {
	if m != nil {
		return m.RangeMaxBytes
	}
	return 0
}

func (m *ZoneConfig) GetGC() *GCPolicy {
	if m != nil {
		return m.GC
	}
	return nil
}

// RangeTree holds the root node and size of the range tree.
type RangeTree struct {
	RootKey          Key    `protobuf:"bytes,1,opt,name=root_key,customtype=Key" json:"root_key"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *RangeTree) Reset()         { *m = RangeTree{} }
func (m *RangeTree) String() string { return proto1.CompactTextString(m) }
func (*RangeTree) ProtoMessage()    {}

// RangeTreeNode holds the configuration for each node of the Red-Black Tree that references all ranges.
type RangeTreeNode struct {
	Key Key `protobuf:"bytes,1,opt,name=key,customtype=Key" json:"key"`
	// Color is black if true, red if false.
	Black bool `protobuf:"varint,2,opt,name=black" json:"black"`
	// If the parent key is null, this is the root node.
	ParentKey        Key    `protobuf:"bytes,3,opt,name=parent_key,customtype=Key" json:"parent_key"`
	LeftKey          *Key   `protobuf:"bytes,4,opt,name=left_key,customtype=Key" json:"left_key,omitempty"`
	RightKey         *Key   `protobuf:"bytes,5,opt,name=right_key,customtype=Key" json:"right_key,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *RangeTreeNode) Reset()         { *m = RangeTreeNode{} }
func (m *RangeTreeNode) String() string { return proto1.CompactTextString(m) }
func (*RangeTreeNode) ProtoMessage()    {}

func (m *RangeTreeNode) GetBlack() bool {
	if m != nil {
		return m.Black
	}
	return false
}

func init() {
}
func (m *RangeTreeNode) Unmarshal(data []byte) error {
	l := len(data)
	index := 0
	for index < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if index >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[index]
			index++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Key", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := index + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Key.Unmarshal(data[index:postIndex]); err != nil {
				return err
			}
			index = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Black", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				v |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Black = bool(v != 0)
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ParentKey", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := index + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.ParentKey.Unmarshal(data[index:postIndex]); err != nil {
				return err
			}
			index = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field LeftKey", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := index + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.LeftKey = &Key{}
			if err := m.LeftKey.Unmarshal(data[index:postIndex]); err != nil {
				return err
			}
			index = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field RightKey", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if index >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[index]
				index++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			postIndex := index + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.RightKey = &Key{}
			if err := m.RightKey.Unmarshal(data[index:postIndex]); err != nil {
				return err
			}
			index = postIndex
		default:
			var sizeOfWire int
			for {
				sizeOfWire++
				wire >>= 7
				if wire == 0 {
					break
				}
			}
			index -= sizeOfWire
			skippy, err := github_com_gogo_protobuf_proto.Skip(data[index:])
			if err != nil {
				return err
			}
			if (index + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, data[index:index+skippy]...)
			index += skippy
		}
	}
	return nil
}
func (m *RangeTreeNode) Size() (n int) {
	var l int
	_ = l
	l = m.Key.Size()
	n += 1 + l + sovConfig(uint64(l))
	n += 2
	l = m.ParentKey.Size()
	n += 1 + l + sovConfig(uint64(l))
	if m.LeftKey != nil {
		l = m.LeftKey.Size()
		n += 1 + l + sovConfig(uint64(l))
	}
	if m.RightKey != nil {
		l = m.RightKey.Size()
		n += 1 + l + sovConfig(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovConfig(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozConfig(x uint64) (n int) {
	return sovConfig(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *RangeTreeNode) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *RangeTreeNode) MarshalTo(data []byte) (n int, err error) {
	var i int
	_ = i
	var l int
	_ = l
	data[i] = 0xa
	i++
	i = encodeVarintConfig(data, i, uint64(m.Key.Size()))
	n1, err := m.Key.MarshalTo(data[i:])
	if err != nil {
		return 0, err
	}
	i += n1
	data[i] = 0x10
	i++
	if m.Black {
		data[i] = 1
	} else {
		data[i] = 0
	}
	i++
	data[i] = 0x1a
	i++
	i = encodeVarintConfig(data, i, uint64(m.ParentKey.Size()))
	n2, err := m.ParentKey.MarshalTo(data[i:])
	if err != nil {
		return 0, err
	}
	i += n2
	if m.LeftKey != nil {
		data[i] = 0x22
		i++
		i = encodeVarintConfig(data, i, uint64(m.LeftKey.Size()))
		n3, err := m.LeftKey.MarshalTo(data[i:])
		if err != nil {
			return 0, err
		}
		i += n3
	}
	if m.RightKey != nil {
		data[i] = 0x2a
		i++
		i = encodeVarintConfig(data, i, uint64(m.RightKey.Size()))
		n4, err := m.RightKey.MarshalTo(data[i:])
		if err != nil {
			return 0, err
		}
		i += n4
	}
	if m.XXX_unrecognized != nil {
		i += copy(data[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func encodeFixed64Config(data []byte, offset int, v uint64) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	data[offset+4] = uint8(v >> 32)
	data[offset+5] = uint8(v >> 40)
	data[offset+6] = uint8(v >> 48)
	data[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32Config(data []byte, offset int, v uint32) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintConfig(data []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		data[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	data[offset] = uint8(v)
	return offset + 1
}
