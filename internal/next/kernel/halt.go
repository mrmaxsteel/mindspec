package kernel

func init() {
	if err := RegisterRule("halt", Halt, finishHalt); err != nil {
		panic(err)
	}
}

// finishHalt closes children before parents and preserves the halt witness.
func finishHalt(s State) (Changes, error) {
	c := Changes{}
	answerID := ""
	answers := s.Answers()
	for _, id := range s.Steps {
		if answers[id].Option == "halt" {
			answerID = id
			break
		}
	}
	if answerID == "" {
		return c, nil
	}
	ids := append(append([]string{}, s.Steps...), s.Gates...)
	for progress := true; progress; {
		progress = false
		for _, id := range ids {
			r := s.Records[id]
			if r.Status == "closed" || hasOpenChild(s, id) || hasOpenBlocker(s, id) {
				continue
			}
			receipt := Receipt{Kind: "halted", HaltAnswer: answerID}
			if answer, ok := answers[id]; ok {
				receipt.Answer = &answer
			}
			w := Write{Kind: "close", ID: id, Receipt: &receipt}
			c.Writes = append(c.Writes, w)
			if err := overlay(&s, w); err != nil {
				return c, err
			}
			progress = true
		}
	}
	return c, nil
}
