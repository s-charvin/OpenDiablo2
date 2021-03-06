package d2asset

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2data"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// Composite is a composite entity animation
type Composite struct {
	baseType    d2enum.ObjectType
	basePath    string
	token       string
	palettePath string
	direction   int
	equipment   [d2enum.CompositeTypeMax]string
	mode        *compositeMode
}

// CreateComposite creates a Composite from a given ObjectLookupRecord and palettePath.
func CreateComposite(baseType d2enum.ObjectType, token, palettePath string) *Composite {
	return &Composite{baseType: baseType, basePath: baseString(baseType),
		token: token, palettePath: palettePath}
}

// Advance moves the composite animation forward for a given elapsed time in nanoseconds.
func (c *Composite) Advance(elapsed float64) error {
	if c.mode == nil {
		return nil
	}

	c.mode.lastFrameTime += elapsed
	framesToAdd := int(c.mode.lastFrameTime / c.mode.animationSpeed)
	c.mode.lastFrameTime -= float64(framesToAdd) * c.mode.animationSpeed
	c.mode.frameIndex += framesToAdd
	c.mode.playedCount += c.mode.frameIndex / c.mode.frameCount
	c.mode.frameIndex %= c.mode.frameCount

	for _, layer := range c.mode.layers {
		if layer != nil {
			if err := layer.Advance(elapsed); err != nil {
				return err
			}
		}
	}

	return nil
}

// Render performs drawing of the Composite on the rendered d2interface.Surface.
func (c *Composite) Render(target d2interface.Surface) error {
	if c.mode == nil {
		return nil
	}

	direction := d2cof.Dir64ToCof(c.direction, c.mode.cof.NumberOfDirections)

	for _, layerIndex := range c.mode.cof.Priority[direction][c.mode.frameIndex] {
		layer := c.mode.layers[layerIndex]
		if layer != nil {
			if err := layer.RenderFromOrigin(target, true); err != nil {
				return err
			}
		}
	}

	for _, layerIndex := range c.mode.cof.Priority[direction][c.mode.frameIndex] {
		layer := c.mode.layers[layerIndex]
		if layer != nil {
			if err := layer.RenderFromOrigin(target, false); err != nil {
				return err
			}
		}
	}

	return nil
}

// ObjectAnimationMode returns the object animation mode
func (c *Composite) ObjectAnimationMode() d2enum.ObjectAnimationMode {
	return c.mode.animationMode.(d2enum.ObjectAnimationMode)
}

// GetAnimationMode returns the animation mode the Composite should render with.
func (c *Composite) GetAnimationMode() string {
	return c.mode.animationMode.String()
}

// GetWeaponClass returns the currently loaded weapon class
func (c *Composite) GetWeaponClass() string {
	return c.mode.weaponClass
}

// SetMode sets the Composite's animation mode weapon class and direction
func (c *Composite) SetMode(animationMode animationMode, weaponClass string) error {
	if c.mode != nil && c.mode.animationMode.String() == animationMode.String() && c.mode.weaponClass == weaponClass {
		return nil
	}

	mode, err := c.createMode(animationMode, weaponClass)
	if err != nil {
		return err
	}

	c.resetPlayedCount()
	c.mode = mode

	return nil
}

// Equip changes the current layer configuration
func (c *Composite) Equip(equipment *[d2enum.CompositeTypeMax]string) error {
	c.equipment = *equipment
	if c.mode == nil {
		return nil
	}

	mode, err := c.createMode(c.mode.animationMode, c.mode.weaponClass)

	if err != nil {
		return err
	}

	c.mode = mode

	return nil
}

// SetAnimSpeed sets the speed at which the Composite's animation should advance through its frames
func (c *Composite) SetAnimSpeed(speed int) {
	c.mode.animationSpeed = 1.0 / ((float64(speed) * 25.0) / 256.0)
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			layer.SetPlaySpeed(c.mode.animationSpeed)
		}
	}
}

// SetDirection sets the direction of the composite and its layers
func (c *Composite) SetDirection(direction int) {
	c.direction = direction
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			if err := layer.SetDirection(c.direction); err != nil {
				fmt.Printf("failed to set direction of layer: %d, err: %v\n", layerIdx, err)
			}
		}
	}
}

// GetDirection returns the current direction the composite is facing
func (c *Composite) GetDirection() int {
	return c.direction
}

// GetPlayedCount returns the number of times the current animation mode has completed all its distinct frames
func (c *Composite) GetPlayedCount() int {
	if c.mode == nil {
		return 0
	}

	return c.mode.playedCount
}

// SetPlayLoop turns on or off animation looping
func (c *Composite) SetPlayLoop(loop bool) {
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			layer.SetPlayLoop(loop)
		}
	}
}

// SetSubLoop sets a loop to be between the specified frame indices
func (c *Composite) SetSubLoop(startFrame, endFrame int) {
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			layer.SetSubLoop(startFrame, endFrame)
		}
	}
}

// SetCurrentFrame sets the current frame index of the animation
func (c *Composite) SetCurrentFrame(frame int) {
	for layerIdx := range c.mode.layers {
		layer := c.mode.layers[layerIdx]
		if layer != nil {
			if err := layer.SetCurrentFrame(frame); err != nil {
				fmt.Printf("failed to set current frame of layer: %d, err: %v\n", layerIdx, err)
			}
		}
	}
}

func (c *Composite) resetPlayedCount() {
	if c.mode != nil {
		c.mode.playedCount = 0
	}
}

type animationMode interface {
	String() string
}

type compositeMode struct {
	cof           *d2cof.COF
	animationMode animationMode
	weaponClass   string
	playedCount   int

	layers []d2interface.Animation

	frameCount     int
	frameIndex     int
	animationSpeed float64
	lastFrameTime  float64
}

func (c *Composite) createMode(animationMode animationMode, weaponClass string) (*compositeMode, error) {
	cofPath := fmt.Sprintf("%s/%s/COF/%s%s%s.COF", c.basePath, c.token, c.token, animationMode, weaponClass)
	if exists, _ := FileExists(cofPath); !exists {
		return nil, errors.New("composite not found")
	}

	cof, err := loadCOF(cofPath)
	if err != nil {
		return nil, err
	}

	animationKey := strings.ToLower(c.token + animationMode.String() + weaponClass)

	animationData := d2data.AnimationData[animationKey]
	if len(animationData) == 0 {
		return nil, errors.New("could not find animation data")
	}

	mode := &compositeMode{
		cof:            cof,
		animationMode:  animationMode,
		weaponClass:    weaponClass,
		layers:         make([]d2interface.Animation, d2enum.CompositeTypeMax),
		frameCount:     animationData[0].FramesPerDirection,
		animationSpeed: 1.0 / ((float64(animationData[0].AnimationSpeed) * 25.0) / 256.0),
	}

	for _, cofLayer := range cof.CofLayers {
		layerValue := c.equipment[cofLayer.Type]
		if layerValue == "" {
			layerValue = "lit"
		}

		drawEffect := d2enum.DrawEffectNone

		if cofLayer.Transparent {
			drawEffect = cofLayer.DrawEffect
		}

		layer, err := c.loadCompositeLayer(cofLayer.Type.String(), layerValue, animationMode.String(),
			cofLayer.WeaponClass.String(), c.palettePath, drawEffect)
		if err == nil {
			layer.SetPlaySpeed(mode.animationSpeed)
			layer.PlayForward()

			if err := layer.SetDirection(c.direction); err != nil {
				return nil, err
			}

			mode.layers[cofLayer.Type] = layer
		}
	}

	return mode, nil
}

func (c *Composite) loadCompositeLayer(layerKey, layerValue, animationMode, weaponClass,
	palettePath string, drawEffect d2enum.DrawEffect) (d2interface.Animation, error) {
	animationPaths := []string{
		fmt.Sprintf("%s/%s/%s/%s%s%s%s%s.dcc", c.basePath, c.token, layerKey, c.token, layerKey, layerValue, animationMode, weaponClass),
		fmt.Sprintf("%s/%s/%s/%s%s%s%s%s.dc6", c.basePath, c.token, layerKey, c.token, layerKey, layerValue, animationMode, weaponClass),
	}

	for _, animationPath := range animationPaths {
		if exists, _ := FileExists(animationPath); exists {
			animation, err := LoadAnimationWithEffect(animationPath, palettePath, drawEffect)
			if err == nil {
				return animation, nil
			}
		}
	}

	return nil, errors.New("animation not found")
}

func baseString(baseType d2enum.ObjectType) string {
	switch baseType {
	case d2enum.ObjectTypePlayer:
		return "/data/global/chars"
	case d2enum.ObjectTypeCharacter:
		return "/data/global/monsters"
	case d2enum.ObjectTypeItem:
		return "/data/global/objects"
	default:
		return ""
	}
}
