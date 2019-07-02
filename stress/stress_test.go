package stress

import (
	"fmt"
	"testing"
	"time"
)

func assertEqual(t *testing.T, expect interface{}, actual interface{}, message string) {
	if expect == actual {
		return
	}
	if len(message) == 0 {
		message = fmt.Sprintf("expect %v !=  actual %v", expect, actual)
	}
	t.Fatal(message)
}

func assertNotEqual(t *testing.T, expect interface{}, actual interface{}, message string) {
	if expect != actual {
		return
	}
	if len(message) == 0 {
		message = fmt.Sprintf("expect %v ==  actual %v", expect, actual)
	}
	t.Fatal(message)
}

func TestCheckBody(t *testing.T) {
	assertEqual(t, nil, CheckBody([]byte("123"), ApiExpect{
		BodyExpectType: EXPECT_EQUAL,
		BodyExp:        "123",
	}), "")

	assertEqual(t, nil, CheckBody([]byte("123"), ApiExpect{
		BodyExpectType: EXPECT_REG,
		BodyExp:        "[0-9]+",
	}), "")

	assertNotEqual(t, nil, CheckBody([]byte("a23"), ApiExpect{
		BodyExpectType: EXPECT_REG,
		BodyExp:        "^[0-9]+$",
	}), "")

	assertEqual(t, nil, CheckBody([]byte(`{"errno": 0}`), ApiExpect{
		BodyExpectType: EXPECT_PCP,
		BodyExp:        `["==", ["prop", ["getJson"], "errno"], 0]`,
	}), "")
}

func TestConer(t *testing.T) {
	coner := GetConer(10)

	for i := 0; i < 10; i++ {
		assertEqual(t, true, coner.Run(func() {
			time.Sleep(5 * time.Millisecond)
		}), "")
	}

	for i := 0; i < 10; i++ {
		assertEqual(t, false, coner.Run(func() {
			time.Sleep(5 * time.Millisecond)
		}), "")
	}

	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 10; i++ {
		assertEqual(t, true, coner.Run(func() {
			time.Sleep(5 * time.Millisecond)
		}), "")
	}
}
