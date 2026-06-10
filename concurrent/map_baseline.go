package concurrent

type MapBaseline struct {
	data map[string]string
}

func NewMapBaseline() *MapBaseline {
	return &MapBaseline{data: make(map[string]string)}
}

func (m *MapBaseline) Size() int {
	return len(m.data)
}

func (m *MapBaseline) Get(key string) (string, bool) {
	v, ok := m.data[key]
	return v, ok
}

func (m *MapBaseline) Put(key string, value string) {
	m.data[key] = value
}
