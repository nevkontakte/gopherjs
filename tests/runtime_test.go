//go:build js && gopherjs

package tests

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gopherjs/gopherjs/js"
)

func Test_parseCallFrame(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Chrome 96.0.4664.110 on Linux #1",
			input: "at foo (eval at $b (https://gopherjs.github.io/playground/playground.js:102:11836), <anonymous>:25887:60)",
			want:  "foo https://gopherjs.github.io/playground/playground.js 102",
		},
		{
			name: "Chrome 96, anonymous eval",
			input: "	at eval (<anonymous>)",
			want: "eval <anonymous> 0",
		},
		{
			name: "Chrome 96, anonymous Array.forEach",
			input: "	at Array.forEach (<anonymous>)",
			want: "Array.forEach <anonymous> 0",
		},
		{
			name:  "Chrome 96, file location only",
			input: "at https://ajax.googleapis.com/ajax/libs/angularjs/1.2.18/angular.min.js:31:225",
			want:  "<none> https://ajax.googleapis.com/ajax/libs/angularjs/1.2.18/angular.min.js 31",
		},
		{
			name:  "Chrome 96, aliased function",
			input: "at k.e.$externalizeWrapper.e.$externalizeWrapper [as run] (https://gopherjs.github.io/playground/playground.js:5:30547)",
			want:  "run https://gopherjs.github.io/playground/playground.js 5",
		},
		{
			name:  "Node.js v12.22.5",
			input: "    at Script.runInThisContext (vm.js:120:18)",
			want:  "Script.runInThisContext vm.js 120",
		},
		{
			name:  "Node.js v12.22.5, aliased function",
			input: "at REPLServer.runBound [as eval] (domain.js:440:12)",
			want:  "eval domain.js 440",
		},
		{
			name:  "Firefox 78.15.0esr Linux",
			input: "getEvalResult@resource://devtools/server/actors/webconsole/eval-with-debugger.js:231:24",
			want:  "getEvalResult resource://devtools/server/actors/webconsole/eval-with-debugger.js 231",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := js.Global.Get("String").New(tt.input)
			frame := runtime.ParseCallFrame(lines)
			got := fmt.Sprintf("%v %v %v", frame.FuncName, frame.File, frame.Line)
			if tt.want != got {
				t.Errorf("Unexpected result: %s", got)
			}
		})
	}
}

func TestBuildPlatform(t *testing.T) {
	if runtime.GOOS != "js" {
		t.Errorf("Got runtime.GOOS=%q. Want: %q.", runtime.GOOS, "js")
	}
	if runtime.GOARCH != "ecmascript" {
		t.Errorf("Got runtime.GOARCH=%q. Want: %q.", runtime.GOARCH, "ecmascript")
	}
}

type callStack []string

func (c *callStack) capture() {
	*c = nil
	pc := [100]uintptr{}
	depth := runtime.Callers(0, pc[:])
	frames := runtime.CallersFrames(pc[:depth])
	for true {
		frame, more := frames.Next()
		*c = append(*c, frame.Function)
		if !more {
			break
		}
	}
}

func TestCallers(t *testing.T) {
	got := callStack{}

	// Expected GopherJS call stack with the upstream Go counterparts.
	// Until https://github.com/gopherjs/gopherjs/issues/1085 is resolved, the
	// mismatch is difficult to avoid.
	want := callStack{
		"Object.Callers", // runtime.Callers
		"typ.$packages.github.com/gopherjs/gopherjs/tests.$ptrType.capture", // github.com/gopherjs/gopherjs/tests.(*callerNames).capture
		"$b",             // github.com/gopherjs/gopherjs/tests.TestCallers.func{1,2}
		"tRunner",        // testing.tRunner
		"runtime.goexit", // runtime.goexit
	}

	t.Run("Normal", func(t *testing.T) {
		got.capture()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("runtime.Callers() returned a diff (-want,+got):\n%s", diff)
		}
	})

	t.Run("Deferred", func(t *testing.T) {
		defer func() {
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("runtime.Callers() returned a diff (-want,+got):\n%s", diff)
			}
		}()
		defer got.capture()
	})

	t.Run("Recover", func(t *testing.T) {
		defer func() {
			recover()
			got.capture()

			want := callStack{
				"Object.Callers", // runtime.Callers
				"typ.$packages.github.com/gopherjs/gopherjs/tests.$ptrType.capture", // github.com/gopherjs/gopherjs/tests.(*callerNames).capture
				"$b",              // github.com/gopherjs/gopherjs/tests.TestCallers.func3.1
				"runtime.gopanic", // runtime.gopanic
				"$b",              // github.com/gopherjs/gopherjs/tests.TestCallers.func{1,2}
				"tRunner",         // testing.tRunner
				"runtime.goexit",  // runtime.goexit
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("runtime.Callers() returned a diff (-want,+got):\n%s", diff)
			}
		}()
		panic("panic")
	})
}
