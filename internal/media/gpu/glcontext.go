//go:build native_media

package gpu

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

var (
	glfwInitialized bool
	initMu          sync.Once
)

func initGLFW() error {
	var err error
	initMu.Do(func() {
		err = glfw.Init()
		glfwInitialized = err == nil
	})
	return err
}

type GLContext struct {
	window    *glfw.Window
	width     int
	height    int
	sharedCtx *glfw.Window
	program   uint32
	texture   uint32
	vao       uint32
	vbo       uint32
	mu        sync.Mutex
}

func NewGLContext(width, height int) (*GLContext, error) {
	if err := initGLFW(); err != nil {
		return nil, fmt.Errorf("failed to init GLFW: %v", err)
	}

	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.Visible, glfw.False)
	glfw.WindowHint(glfw.DepthBits, 0)
	glfw.WindowHint(glfw.StencilBits, 0)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(width, height, "VideoTools GPU", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GLFW window: %v", err)
	}

	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		window.Destroy()
		return nil, fmt.Errorf("failed to init GL: %v", err)
	}

	ctx := &GLContext{
		window: window,
		width:  width,
		height: height,
	}

	if err := ctx.initShaders(); err != nil {
		ctx.Delete()
		return nil, err
	}

	if err := ctx.initBuffers(); err != nil {
		ctx.Delete()
		return nil, err
	}

	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	return ctx, nil
}

func (c *GLContext) initShaders() error {
	vertexShader := `#version 120
attribute vec2 aPosition;
attribute vec2 aTexCoord;
varying vec2 vTexCoord;
void main() {
    gl_Position = vec4(aPosition, 0.0, 1.0);
    vTexCoord = aTexCoord;
}`

	fragmentShader := `#version 120
precision mediump float;
uniform sampler2D uTexture;
varying vec2 vTexCoord;
void main() {
    gl_FragColor = texture2D(uTexture, vTexCoord);
}`

	vs, err := compileShader(vertexShader, gl.VERTEX_SHADER)
	if err != nil {
		return err
	}
	defer gl.DeleteShader(vs)

	fs, err := compileShader(fragmentShader, gl.FRAGMENT_SHADER)
	if err != nil {
		return err
	}
	defer gl.DeleteShader(fs)

	program := gl.CreateProgram()
	gl.AttachShader(program, vs)
	gl.AttachShader(program, fs)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetProgramInfoLog(program, logLen, nil, &log[0])
		gl.DeleteProgram(program)
		return fmt.Errorf("shader link failed: %s", string(log))
	}

	c.program = program
	return nil
}

func (c *GLContext) initBuffers() error {
	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)
	c.vao = vao

	quad := []float32{
		-1, -1, 0, 1,
		1, -1, 1, 1,
		-1, 1, 0, 0,
		1, 1, 1, 0,
	}

	var vbo uint32
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(quad)*4, gl.Ptr(quad), gl.STATIC_DRAW)
	c.vbo = vbo

	posLoc := gl.GetAttribLocation(c.program, gl.Str("aPosition\x00"))
	texLoc := gl.GetAttribLocation(c.program, gl.Str("aTexCoord\x00"))

	gl.EnableVertexAttribArray(uint32(posLoc))
	gl.VertexAttribPointer(uint32(posLoc), 2, gl.FLOAT, false, 16, unsafe.Pointer(nil))

	gl.EnableVertexAttribArray(uint32(texLoc))
	gl.VertexAttribPointer(uint32(texLoc), 2, gl.FLOAT, false, 16, unsafe.Pointer(unsafe.Sizeof(float32(0))*2))

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	c.texture = tex

	gl.BindVertexArray(0)

	return nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)
	sourceChars, freeFn := gl.Strs(source + "\x00")
	defer freeFn()

	gl.ShaderSource(shader, 1, sourceChars, nil)
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLen int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLen)
		log := make([]byte, logLen)
		gl.GetShaderInfoLog(shader, logLen, nil, &log[0])
		gl.DeleteShader(shader)
		return 0, fmt.Errorf("shader compile failed: %s", string(log))
	}

	return shader, nil
}

func (c *GLContext) MakeCurrent() {
	if c.window != nil {
		c.window.MakeContextCurrent()
	}
}

func (c *GLContext) SwapBuffers() {
	if c.window != nil {
		c.window.SwapBuffers()
	}
}

func (c *GLContext) Resize(width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.width = width
	c.height = height
}

func (c *GLContext) UploadTexture(data []byte, width, height int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.texture == 0 {
		return fmt.Errorf("no texture created")
	}

	gl.BindTexture(gl.TEXTURE_2D, c.texture)
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(width),
		int32(height),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		unsafe.Pointer(&data[0]),
	)

	return nil
}

func (c *GLContext) Render() {
	c.mu.Lock()
	defer c.mu.Unlock()

	gl.Viewport(0, 0, int32(c.width), int32(c.height))
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	if c.texture == 0 || c.program == 0 {
		return
	}

	gl.UseProgram(c.program)
	gl.BindTexture(gl.TEXTURE_2D, c.texture)
	gl.BindVertexArray(c.vao)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)
}

func (c *GLContext) Delete() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.vbo > 0 {
		gl.DeleteBuffers(1, &c.vbo)
		c.vbo = 0
	}
	if c.vao > 0 {
		gl.DeleteVertexArrays(1, &c.vao)
		c.vao = 0
	}
	if c.texture > 0 {
		gl.DeleteTextures(1, &c.texture)
		c.texture = 0
	}
	if c.program > 0 {
		gl.DeleteProgram(c.program)
		c.program = 0
	}
	if c.window != nil {
		c.window.Destroy()
		c.window = nil
	}
}

func (c *GLContext) ShouldClose() bool {
	if c.window == nil {
		return true
	}
	return c.window.ShouldClose()
}

func (c *GLContext) PollEvents() {
	glfw.PollEvents()
}
