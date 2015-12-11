package keys

import (
	"github.com/dmatrixdb/dmatrix/roachpb"
	"github.com/dmatrixdb/dmatrix/util/encoding"
)

// MakeKey makes a new key which is the concatenation of the
// given inputs, in order.
func MakeKey(keys ...[]byte) []byte {
	return roachpb.MakeKey(keys...)
}

// RaftLogKey returns a system-local key for a Raft log entry.
func RaftLogKey(rangeID roachpb.RangeID, logIndex uint64) roachpb.Key {
	return MakeRangeIDKey(rangeID, localRaftLogSuffix,
		encoding.EncodeUint64(nil, logIndex))
}

// RaftLogPrefix returns the system-local prefix shared by all entries in a Raft log.
func RaftLogPrefix(rangeID roachpb.RangeID) roachpb.Key {
	return MakeRangeIDKey(rangeID, localRaftLogSuffix, roachpb.RKey{})
}

// RaftHardStateKey returns a system-local key for a Raft HardState.
func RaftHardStateKey(rangeID roachpb.RangeID) roachpb.Key {
	return MakeRangeIDKey(rangeID, localRaftHardStateSuffix, roachpb.RKey{})
}

// RaftTruncatedStateKey returns a system-local key for a RaftTruncatedState.
func RaftTruncatedStateKey(rangeID roachpb.RangeID) roachpb.Key {
	return MakeRangeIDKey(rangeID, localRaftTruncatedStateSuffix, roachpb.RKey{})
}

// RaftAppliedIndexKey returns a system-local key for a raft applied index.
func RaftAppliedIndexKey(rangeID roachpb.RangeID) roachpb.Key {
	return MakeRangeIDKey(rangeID, localRaftAppliedIndexSuffix, roachpb.RKey{})
}

// RaftLeaderLeaseKey returns a system-local key for a raft leader lease.
func RaftLeaderLeaseKey(rangeID roachpb.RangeID) roachpb.Key {
	return MakeRangeIDKey(rangeID, localRaftLeaderLeaseSuffix, roachpb.RKey{})
}

// RaftTombstoneKey returns a system-local key for a raft tombstone.
func RaftTombstoneKey(rangeID roachpb.RangeID) roachpb.Key {
	return MakeRangeIDKey(rangeID, localRaftTombstoneSuffix, roachpb.RKey{})
}

// RaftLastIndexKey returns a system-local key for a raft last index.
func RaftLastIndexKey(rangeID roachpb.RangeID) roachpb.Key {
	return MakeRangeIDKey(rangeID, localRaftLastIndexSuffix, roachpb.RKey{})
}
