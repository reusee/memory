package main

type Memory struct {
	Concepts map[string]*Concept
	Connects map[string]*Connect
	Serial   int
}

func (self *Memory) AddConcept(concept *Concept) (exists bool) {
	if _, exists = self.Concepts[concept.Key()]; exists {
		return
	}
	self.Concepts[concept.Key()] = concept
	return
}

func (self *Memory) AddConnect(connect *Connect) (exists bool) {
	if _, exists = self.Connects[connect.Key()]; exists {
		return
	}
	self.Connects[connect.Key()] = connect
	return
}

func (self *Memory) NextSerial() int {
	self.Serial += 1
	return self.Serial
}
