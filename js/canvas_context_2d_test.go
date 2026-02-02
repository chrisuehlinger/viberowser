package js

import (
	"testing"

	"github.com/chrisuehlinger/viberowser/dom"
)

func TestCanvasBasic(t *testing.T) {
	// Create a document with a canvas element
	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html>
<head></head>
<body><canvas id="canvas"></canvas></body>
</html>`)

	// Create JS runtime and bind the document
	r := NewRuntime()
	binder := NewDOMBinder(r)
	binder.BindDocument(doc)

	// Test basic canvas properties
	result, err := r.Execute(`
		var canvas = document.getElementById('canvas');
		var results = [];

		// Test default dimensions
		results.push('width: ' + canvas.width);
		results.push('height: ' + canvas.height);

		// Set dimensions
		canvas.width = 400;
		canvas.height = 300;
		results.push('new width: ' + canvas.width);
		results.push('new height: ' + canvas.height);

		// Get 2D context
		var ctx = canvas.getContext('2d');
		results.push('ctx exists: ' + (ctx !== null));
		results.push('ctx.canvas === canvas: ' + (ctx.canvas === canvas));

		// Test fillStyle
		results.push('default fillStyle: ' + ctx.fillStyle);
		ctx.fillStyle = 'red';
		results.push('red fillStyle: ' + ctx.fillStyle);

		// Test fillRect (shouldn't throw)
		ctx.fillRect(10, 10, 50, 50);
		results.push('fillRect: ok');

		// Test strokeStyle
		ctx.strokeStyle = '#00ff00';
		results.push('green strokeStyle: ' + ctx.strokeStyle);

		// Test path operations
		ctx.beginPath();
		ctx.moveTo(0, 0);
		ctx.lineTo(100, 100);
		ctx.stroke();
		results.push('path: ok');

		// Test measureText
		ctx.font = '16px Arial';
		var metrics = ctx.measureText('Hello');
		results.push('measureText width: ' + (metrics.width > 0));

		// Test save/restore
		ctx.save();
		ctx.fillStyle = 'blue';
		results.push('blue fillStyle: ' + ctx.fillStyle);
		ctx.restore();
		results.push('restored fillStyle: ' + ctx.fillStyle);

		results.join('\n');
	`)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	expected := `width: 300
height: 150
new width: 400
new height: 300
ctx exists: true
ctx.canvas === canvas: true
default fillStyle: #000000
red fillStyle: #ff0000
fillRect: ok
green strokeStyle: #00ff00
path: ok
measureText width: true
blue fillStyle: #0000ff
restored fillStyle: #ff0000`

	if result.String() != expected {
		t.Errorf("Unexpected result:\nGot:\n%s\n\nExpected:\n%s", result.String(), expected)
	}
}

func TestCanvasContextIdentity(t *testing.T) {
	// Test that getContext returns the same object on multiple calls
	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html><body><canvas id="canvas"></canvas></body></html>`)

	r := NewRuntime()
	binder := NewDOMBinder(r)
	binder.BindDocument(doc)

	result, err := r.Execute(`
		var canvas = document.getElementById('canvas');
		var ctx1 = canvas.getContext('2d');
		var ctx2 = canvas.getContext('2d');
		ctx1 === ctx2;
	`)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	if result.ToBoolean() != true {
		t.Error("Expected getContext to return the same object on multiple calls")
	}
}

func TestCanvasUnsupportedContexts(t *testing.T) {
	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html><body><canvas id="canvas"></canvas></body></html>`)

	r := NewRuntime()
	binder := NewDOMBinder(r)
	binder.BindDocument(doc)

	// Test that unsupported context types return null
	result, err := r.Execute(`
		var canvas = document.getElementById('canvas');
		var results = [];
		results.push('webgl: ' + (canvas.getContext('webgl') === null));
		results.push('webgl2: ' + (canvas.getContext('webgl2') === null));
		results.push('invalid: ' + (canvas.getContext('invalid') === null));
		results.join('\n');
	`)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	expected := `webgl: true
webgl2: true
invalid: true`

	if result.String() != expected {
		t.Errorf("Unexpected result:\nGot:\n%s\n\nExpected:\n%s", result.String(), expected)
	}
}

func TestCanvasTransformations(t *testing.T) {
	doc, _ := dom.ParseHTML(`<!DOCTYPE html>
<html><body><canvas id="canvas"></canvas></body></html>`)

	r := NewRuntime()
	binder := NewDOMBinder(r)
	binder.BindDocument(doc)

	result, err := r.Execute(`
		var canvas = document.getElementById('canvas');
		var ctx = canvas.getContext('2d');
		var results = [];

		// Test getTransform
		var t = ctx.getTransform();
		results.push('initial: a=' + t.a + ' b=' + t.b + ' c=' + t.c + ' d=' + t.d + ' e=' + t.e + ' f=' + t.f);

		// Test translate
		ctx.translate(10, 20);
		t = ctx.getTransform();
		results.push('after translate: e=' + t.e + ' f=' + t.f);

		// Test resetTransform
		ctx.resetTransform();
		t = ctx.getTransform();
		results.push('after reset: a=' + t.a + ' d=' + t.d + ' e=' + t.e + ' f=' + t.f);

		results.join('\n');
	`)
	if err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	expected := `initial: a=1 b=0 c=0 d=1 e=0 f=0
after translate: e=10 f=20
after reset: a=1 d=1 e=0 f=0`

	if result.String() != expected {
		t.Errorf("Unexpected result:\nGot:\n%s\n\nExpected:\n%s", result.String(), expected)
	}
}
