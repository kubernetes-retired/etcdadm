package pki

import "sync"

type InMemoryMutableKeypair struct {
	Keypair *Keypair
}

var _ MutableKeypair = &InMemoryMutableKeypair{}

func (s *InMemoryMutableKeypair) MutateKeypair(mutator func(keypair *Keypair) error) (*Keypair, error) {
	keypair := &Keypair{}

	if s.Keypair != nil {
		*keypair = *s.Keypair
	}

	if err := mutator(keypair); err != nil {
		return nil, err
	}

	if s.Keypair == nil {
		s.Keypair = &Keypair{}
	}
	*s.Keypair = *keypair
	return keypair, nil
}

type InMemoryStore struct {
	mutex sync.Mutex
	data  map[string]*InMemoryMutableKeypair
}

var _ Store = &InMemoryStore{}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]*InMemoryMutableKeypair),
	}
}

func (s *InMemoryStore) Keypair(name string) MutableKeypair {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	k := s.data[name]
	if k == nil {
		k = &InMemoryMutableKeypair{}
		s.data[name] = k
	}
	return k
}
