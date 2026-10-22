package providers

import "sync"

type rotationState struct{ index, uses int }

var comboRotations = struct {
	sync.Mutex
	states map[string]rotationState
}{states: map[string]rotationState{}}

func RotateCombo(models []string, name, strategy string, sticky int) []string {
	result := append([]string(nil), models...)
	if len(models) <= 1 || strategy != "round-robin" {
		return result
	}
	if sticky <= 0 {
		sticky = 1
	}
	if name == "" {
		name = "__default__"
	}
	comboRotations.Lock()
	defer comboRotations.Unlock()
	state := comboRotations.states[name]
	index := state.index % len(models)
	result = append(append([]string{}, models[index:]...), models[:index]...)
	state.index = index
	state.uses++
	if state.uses >= sticky {
		state.index = (index + 1) % len(models)
		state.uses = 0
	}
	comboRotations.states[name] = state
	return result
}
func ResetComboRotation(name string) {
	comboRotations.Lock()
	defer comboRotations.Unlock()
	if name == "" {
		comboRotations.states = map[string]rotationState{}
	} else {
		delete(comboRotations.states, name)
	}
}
