// Copyright 2015 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.
//
// Author: Bram Gruneir (bram.gruneir@gmail.com)

package storage_test

import (
	"reflect"
	"testing"

	"github.com/cockroachdb/cockroach/client"
	"github.com/cockroachdb/cockroach/proto"
	"github.com/cockroachdb/cockroach/storage/engine"
	"github.com/cockroachdb/cockroach/util"
	"github.com/cockroachdb/cockroach/util/leaktest"
)

type testRangeTree struct {
	Tree  proto.RangeTree
	Nodes map[string]proto.RangeTreeNode
}

// nodesEqual is a replacement for reflect.DeepEqual as it was having issues
// determining similarites between the types.
func nodesEqual(key proto.Key, expected, actual proto.RangeTreeNode) error {
	// Check that Key is equal.
	if !expected.Key.Equal(actual.Key) {
		return util.Errorf("Range tree node's Key is not as expected for range:%s\nexpected:%+v\nactual:%+v", key, expected, actual)
	}
	// Check that Black are equal.
	if expected.Black != actual.Black {
		return util.Errorf("Range tree node's Black is not as expected for range:%s\nexpected:%+v\nactual:%+v", key, expected, actual)
	}
	// Check that LeftKey are equal.
	if (expected.LeftKey == nil) || (actual.LeftKey == nil) {
		if !((expected.LeftKey == nil) && (actual.LeftKey == nil)) {
			return util.Errorf("Range tree node's LeftKey is not as expected for range:%s\nexpected:%+v\nactual:%+v", key, expected, actual)
		}
	} else if !(*expected.LeftKey).Equal(*actual.LeftKey) {
		return util.Errorf("Range tree node's LeftKey is not as expected for range:%s\nexpected:%+v\nactual:%+v", key, expected, actual)
	}
	// Check that RightKey are equal.
	if (expected.RightKey == nil) || (actual.RightKey == nil) {
		if !((expected.RightKey == nil) && (actual.RightKey == nil)) {
			return util.Errorf("Range tree node's RightKey is not as expected for range:%s\nexpected:%+v\nactual:%+v", key, expected, actual)
		}
	} else if !(*expected.RightKey).Equal(*actual.RightKey) {
		return util.Errorf("Range tree node's RightKey is not as expected for range:%s\nexpected:%+v\nactual:%+v", key, expected, actual)
	}
	// Check that PArentKey are equal.
	if !expected.ParentKey.Equal(actual.ParentKey) {
		return util.Errorf("Range tree node's LeftKey is not as expected for range:%s\nexpected:%+v\nactual:%+v", key, expected, actual)
	}
	return nil
}

// treeNodesEqual compares the expectedTree from the provided key to the actual
// nodes retrieved from the db.  It recursively calls itself on both left and
// right children if they exist.
func treeNodesEqual(db *client.KV, expected testRangeTree, key proto.Key) error {
	expectedNode, ok := expected.Nodes[string(key)]
	if !ok {
		return util.Errorf("Expected does not contain a node for %s", key)
	}
	actualNode := &proto.RangeTreeNode{}
	ok, _, err := db.GetProto(engine.RangeTreeNodeKey(key), actualNode)
	if err != nil {
		return err
	}
	if !ok {
		return util.Errorf("Could not find the range tree node for range:%s; expected:%+v", key, expectedNode)
	}
	if err = nodesEqual(key, expectedNode, *actualNode); err != nil {
		return err
	}
	if expectedNode.LeftKey != nil {
		if err = treeNodesEqual(db, expected, *expectedNode.LeftKey); err != nil {
			return err
		}
	}
	if expectedNode.RightKey != nil {
		if err = treeNodesEqual(db, expected, *expectedNode.RightKey); err != nil {
			return err
		}
	}
	return nil
}

// treesEqual compares the expectedTree and expectedNodes to the actual range
// tree stored in the db.
func treesEqual(db *client.KV, expected testRangeTree) error {
	// Compare the tree roots.
	actualTree := &proto.RangeTree{}
	ok, _, err := db.GetProto(engine.KeyRangeTreeRoot, actualTree)
	if err != nil {
		return err
	}
	if !ok {
		return util.Errorf("Could not find the range tree root:%s", engine.KeyRangeTreeRoot)
	}
	if !reflect.DeepEqual(&expected.Tree, actualTree) {
		return util.Errorf("Range tree root is not as expected - expected:%+v - actual:%+v", expected.Tree, actualTree)
	}

	return treeNodesEqual(db, expected, expected.Tree.RootKey)
}

// splitRange splits whichever range contains the split key on the split key.
func splitRange(db *client.KV, key proto.Key, splitKey proto.Key) error {
	req := &proto.AdminSplitRequest{
		RequestHeader: proto.RequestHeader{
			Key: key,
		},
		SplitKey: splitKey,
	}
	resp := &proto.AdminSplitResponse{}
	if err := db.Call(proto.AdminSplit, req, resp); err != nil {
		return err
	}

	return nil
}

// TestSetupRangeTree ensures that SetupRangeTree correctly setups up the range
// tree and first node.  SetupRangeTree is called via store.BootstrapRange.
func TestSetupRangeTree(t *testing.T) {
	defer leaktest.AfterTest(t)
	store := createTestStore(t)
	defer store.Stop()

	// Check to make sure the range tree is stored correctly.
	tree := proto.RangeTree{
		RootKey: engine.KeyMin,
	}
	nodes := map[string]proto.RangeTreeNode{
		string(engine.KeyMin): proto.RangeTreeNode{
			Key:       engine.KeyMin,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
	}
	expectedTree := testRangeTree{
		Tree:  tree,
		Nodes: nodes,
	}
	if err := treesEqual(store.DB(), expectedTree); err != nil {
		t.Fatal(err)
	}
}

// TestInsert tests inserting a collection of 5 nodes, forcing left rotations
// and flips.
func TestInsert(t *testing.T) {
	defer leaktest.AfterTest(t)
	store := createTestStore(t)
	defer store.Stop()
	db := store.DB()

	// Prepare the expected tree
	keyA := proto.Key("a")
	keyB := proto.Key("b")
	keyC := proto.Key("c")
	keyD := proto.Key("d")
	keyE := proto.Key("e")

	tree := proto.RangeTree{
		RootKey: keyA,
	}
	nodes := map[string]proto.RangeTreeNode{
		string(keyA): proto.RangeTreeNode{
			Key:       keyA,
			ParentKey: engine.KeyMin,
			Black:     true,
			LeftKey:   &engine.KeyMin,
		},
		string(engine.KeyMin): proto.RangeTreeNode{
			Key:       engine.KeyMin,
			ParentKey: engine.KeyMin,
			Black:     false,
		},
	}
	expectedTree := testRangeTree{
		Tree:  tree,
		Nodes: nodes,
	}

	// Test single split (with a left rotation).
	if err := splitRange(db, engine.KeyMin, keyA); err != nil {
		t.Fatal(err)
	}
	if err := treesEqual(db, expectedTree); err != nil {
		t.Fatal(err)
	}

	// Test two splits (with a flip).
	tree = proto.RangeTree{
		RootKey: keyA,
	}
	nodes = map[string]proto.RangeTreeNode{
		string(keyA): proto.RangeTreeNode{
			Key:       keyA,
			ParentKey: engine.KeyMin,
			Black:     true,
			LeftKey:   &engine.KeyMin,
			RightKey:  &keyB,
		},
		string(engine.KeyMin): proto.RangeTreeNode{
			Key:       engine.KeyMin,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
		string(keyB): proto.RangeTreeNode{
			Key:       keyB,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
	}
	expectedTree = testRangeTree{
		Tree:  tree,
		Nodes: nodes,
	}
	if err := splitRange(db, keyA, keyB); err != nil {
		t.Fatal(err)
	}
	if err := treesEqual(db, expectedTree); err != nil {
		t.Fatal(err)
	}

	// Test three splits (with a left rotation).
	tree = proto.RangeTree{
		RootKey: keyA,
	}
	nodes = map[string]proto.RangeTreeNode{
		string(keyA): proto.RangeTreeNode{
			Key:       keyA,
			ParentKey: engine.KeyMin,
			Black:     true,
			LeftKey:   &engine.KeyMin,
			RightKey:  &keyC,
		},
		string(engine.KeyMin): proto.RangeTreeNode{
			Key:       engine.KeyMin,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
		string(keyC): proto.RangeTreeNode{
			Key:       keyC,
			ParentKey: engine.KeyMin,
			Black:     true,
			LeftKey:   &keyB,
		},
		string(keyB): proto.RangeTreeNode{
			Key:       keyB,
			ParentKey: engine.KeyMin,
			Black:     false,
		},
	}
	expectedTree = testRangeTree{
		Tree:  tree,
		Nodes: nodes,
	}
	if err := splitRange(db, keyB, keyC); err != nil {
		t.Fatal(err)
	}
	if err := treesEqual(db, expectedTree); err != nil {
		t.Fatal(err)
	}

	// Test four splits (with a flip and left rotation).
	tree = proto.RangeTree{
		RootKey: keyC,
	}
	nodes = map[string]proto.RangeTreeNode{
		string(keyC): proto.RangeTreeNode{
			Key:       keyC,
			ParentKey: engine.KeyMin,
			Black:     true,
			LeftKey:   &keyA,
			RightKey:  &keyD,
		},
		string(keyA): proto.RangeTreeNode{
			Key:       keyA,
			ParentKey: engine.KeyMin,
			Black:     false,
			LeftKey:   &engine.KeyMin,
			RightKey:  &keyB,
		},
		string(engine.KeyMin): proto.RangeTreeNode{
			Key:       engine.KeyMin,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
		string(keyB): proto.RangeTreeNode{
			Key:       keyB,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
		string(keyD): proto.RangeTreeNode{
			Key:       keyD,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
	}
	expectedTree = testRangeTree{
		Tree:  tree,
		Nodes: nodes,
	}
	if err := splitRange(db, keyC, keyD); err != nil {
		t.Fatal(err)
	}
	if err := treesEqual(db, expectedTree); err != nil {
		t.Fatal(err)
	}

	// Test four splits (with a left rotation).
	tree = proto.RangeTree{
		RootKey: keyC,
	}
	nodes = map[string]proto.RangeTreeNode{
		string(keyC): proto.RangeTreeNode{
			Key:       keyC,
			ParentKey: engine.KeyMin,
			Black:     true,
			LeftKey:   &keyA,
			RightKey:  &keyE,
		},
		string(keyA): proto.RangeTreeNode{
			Key:       keyA,
			ParentKey: engine.KeyMin,
			Black:     false,
			LeftKey:   &engine.KeyMin,
			RightKey:  &keyB,
		},
		string(engine.KeyMin): proto.RangeTreeNode{
			Key:       engine.KeyMin,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
		string(keyB): proto.RangeTreeNode{
			Key:       keyB,
			ParentKey: engine.KeyMin,
			Black:     true,
		},
		string(keyE): proto.RangeTreeNode{
			Key:       keyE,
			ParentKey: engine.KeyMin,
			Black:     true,
			LeftKey:   &keyD,
		},
		string(keyD): proto.RangeTreeNode{
			Key:       keyD,
			ParentKey: engine.KeyMin,
			Black:     false,
		},
	}
	expectedTree = testRangeTree{
		Tree:  tree,
		Nodes: nodes,
	}
	if err := splitRange(db, keyD, keyE); err != nil {
		t.Fatal(err)
	}
	if err := treesEqual(db, expectedTree); err != nil {
		t.Fatal(err)
	}
}
