package structs

import (
	"math"
	"sync"
)

type node[K key] struct {
	prev  *node[K]
	next  *node[K]
	keys  *hashSet[K]
	count int
}

func newNode[K key](key K) *node[K] {
	keys := newHashSet[K]()
	keys.insert(key)

	return &node[K]{
		prev:  nil,
		next:  nil,
		keys:  keys,
		count: 1,
	}
}

func newNodeWithCount[K key](key K, count int) *node[K] {
	keys := newHashSet[K]()
	keys.insert(key)

	return &node[K]{
		prev:  nil,
		next:  nil,
		keys:  keys,
		count: count,
	}
}

func newEmptyNodeWithCount[K key](count int) *node[K] {
	keys := newHashSet[K]()

	return &node[K]{
		prev:  nil,
		next:  nil,
		keys:  keys,
		count: count,
	}
}

type linkedList[K key] struct {
	head *node[K]
	tail *node[K]
}

func newLinkedList[K key]() *linkedList[K] {
	var headTailKey K
	head := newNodeWithCount[K](headTailKey, math.MaxInt)
	tail := newNodeWithCount[K](headTailKey, math.MinInt)

	head.next = tail
	tail.prev = head

	return &linkedList[K]{
		head: head,
		tail: tail,
	}
}

/*
	- add new node
	- remove existing node
	- increment count of an existing key
	- decrement count of an existing key
*/

func (l *linkedList[K]) insertBetween(nodeBefore *node[K], nodeAfter *node[K], nodeToInsert *node[K]) {
	nodeToInsert.prev = nodeBefore
	nodeToInsert.next = nodeAfter

	nodeBefore.next = nodeToInsert
	nodeAfter.prev = nodeToInsert
}

func (l *linkedList[K]) insertAfter(insertAfter, nodeToInsert *node[K]) {
	nextNode := insertAfter.next
	l.insertBetween(insertAfter, nextNode, nodeToInsert)
}

func (l *linkedList[K]) insertBefore(insertBefore, nodeToInsert *node[K]) {
	prevNode := insertBefore.prev
	l.insertBetween(prevNode, insertBefore, nodeToInsert)
}

func (l *linkedList[K]) addNewKey(key K) *node[K] {
	if l.tail.prev.count == 1 {
		l.tail.prev.keys.insert(key)
		return l.tail.prev
	}
	nodeToInsert := newNode[K](key)
	l.insertBefore(l.tail, nodeToInsert)
	return nodeToInsert
}

func (l *linkedList[K]) removeExistingNode(node *node[K]) {
	prevNode := node.prev
	nextNode := node.next

	prevNode.next = nextNode
	nextNode.prev = prevNode
}

func (l *linkedList[K]) incrementCountForExistingKey(key K, currNode *node[K]) *node[K] {
	currCount := currNode.count
	newCount := currCount + 1

	if currNode.prev.count != newCount {
		nodeToInsert := newEmptyNodeWithCount[K](newCount)
		l.insertBefore(currNode, nodeToInsert)
	}

	currNode.keys.delete(key)
	currNode.prev.keys.insert(key)

	prevNode := currNode.prev
	if currNode.keys.size() == 0 {
		l.removeExistingNode(currNode)
	}
	return prevNode
}

func (l *linkedList[K]) decrementCountForExistingKey(key K, currNode *node[K]) *node[K] {
	currCount := currNode.count
	newCount := currCount - 1

	if newCount == 0 {
		currNode.keys.delete(key)
		if currNode.keys.size() == 0 {
			l.removeExistingNode(currNode)
		}
		return nil
	}

	if currNode.next.count != newCount {
		nodeToInsert := newEmptyNodeWithCount[K](newCount)
		l.insertAfter(currNode, nodeToInsert)
	}

	currNode.keys.delete(key)
	currNode.next.keys.insert(key)

	nextNode := currNode.next
	if currNode.keys.size() == 0 {
		l.removeExistingNode(currNode)
	}
	return nextNode
}

type MutexAllForOne[K key] struct {
	mu         sync.RWMutex
	linkedList *linkedList[K]
	lookup     map[K]*node[K]
}

func NewAllForOne[K key]() *MutexAllForOne[K] {
	return &MutexAllForOne[K]{
		linkedList: newLinkedList[K](),
		lookup:     make(map[K]*node[K]),
	}
}

func (a *MutexAllForOne[K]) Inc(key K) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if nodeToInc, ok := a.lookup[key]; ok {
		newNodeForKey := a.linkedList.incrementCountForExistingKey(key, nodeToInc)
		a.lookup[key] = newNodeForKey
	} else {
		newNodeForKey := a.linkedList.addNewKey(key)
		a.lookup[key] = newNodeForKey
	}
}

func (a *MutexAllForOne[K]) Dec(key K) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if nodeToDec, ok := a.lookup[key]; ok {
		newNodeForKey := a.linkedList.decrementCountForExistingKey(key, nodeToDec)
		if newNodeForKey == nil {
			delete(a.lookup, key)
		} else {
			a.lookup[key] = newNodeForKey
		}
	} else {
		panic("key not found")
	}
}

func (a *MutexAllForOne[K]) GetLeastRareKey() K {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.linkedList.head.next.keys.getAny()
}

func (a *MutexAllForOne[K]) GetMostRareKey() K {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.linkedList.tail.prev.keys.getAny()
}
