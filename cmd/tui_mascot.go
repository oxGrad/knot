package cmd

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// knotArt is "KNOT" in 6-row block-letter style; each row is 37 visual columns wide.
var knotArt = [6]string{
	`██╗  ██╗███╗   ██╗ ██████╗ ████████╗`,
	`██║ ██╔╝████╗  ██║██╔═══██╗╚══██╔══╝`,
	`█████╔╝ ██╔██╗ ██║██║   ██║   ██║   `,
	`██╔═██╗ ██║╚██╗██║██║   ██║   ██║   `,
	`██║  ██╗██║ ╚████║╚██████╔╝   ██║   `,
	`╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝    ╚═╝   `,
}

// monkeyFrames[state][frame][line] — each line is exactly 8 visual columns.
var monkeyFrames = [3][3][6]string{
	// mascotNormal: peach, blink (frame 1) + grin (frame 2)
	{
		{`  ▄▄▄▄  `, `▐(o  o)▌`, `▐( ▄▄ )▌`, `▐( ~~ )▌`, ` ▀(  )▀ `, `   ██   `},
		{`  ▄▄▄▄  `, `▐(─  ─)▌`, `▐( ▄▄ )▌`, `▐( ~~ )▌`, ` ▀(  )▀ `, `   ██   `},
		{`  ▄▄▄▄  `, `▐(o  o)▌`, `▐( ▄▄ )▌`, `▐( ^^ )▌`, ` ▀(  )▀ `, `   ██   `},
	},
	// mascotConflict: red, fur rises each frame + frantic eyes + bared teeth
	{
		{`  ▄▄▄▄  `, `▐(>  <)▌`, `▐( ▄▄ )▌`, `▐( !! )▌`, ` ▀(  )▀ `, `   ██   `},
		{` ▄▄▄▄▄▄ `, `▐(X  X)▌`, `▐( ▄▄ )▌`, `▐( ## )▌`, ` ▀(  )▀ `, `   ██   `},
		{`▄▄▄▄▄▄▄▄`, `▐(*  *)▌`, `▐( ▄▄ )▌`, `▐( >> )▌`, ` ▀(  )▀ `, `   ██   `},
	},
	// mascotMissing: yellow, eyes dart side-to-side
	{
		{`  ▄▄▄▄  `, `▐(o  .)▌`, `▐( ▄▄ )▌`, `▐( ?? )▌`, ` ▀(  )▀ `, `   ██   `},
		{`  ▄▄▄▄  `, `▐(.  .)▌`, `▐( ▄▄ )▌`, `▐( ?? )▌`, ` ▀(  )▀ `, `   ██   `},
		{`  ▄▄▄▄  `, `▐(.  o)▌`, `▐( ▄▄ )▌`, `▐( ?? )▌`, ` ▀(  )▀ `, `   ██   `},
	},
}

// jellyfishFrames[state][frame][line] — each line is exactly 8 visual columns.
// Tentacle lines use "|" as a colour-split marker (left=blue, right=red).
var jellyfishFrames = [3][3][6]string{
	{
		{` ▄████▄ `, `▐ o  o ▌`, `▐  ~~  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
		{` ▄████▄ `, `▐ ─  ─ ▌`, `▐  ~~  ▌`, ` ▀████▀ `, `╵╷╵╷|╵╿╵╿`, `│ │ |║ ║ `},
		{` ▄████▄ `, `▐ o  o ▌`, `▐  ^^  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
	},
	{
		{` ▄████▄ `, `▐ >  < ▌`, `▐  !!  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
		{`▄██████▄`, `▐ X  X ▌`, `▐  ##  ▌`, ` ▀████▀ `, `╵╷╵╷|╵╿╵╿`, `│ │ |║ ║ `},
		{`████████`, `▐ *  * ▌`, `▐  >>  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
	},
	{
		{` ▄████▄ `, `▐ o  . ▌`, `▐  ??  ▌`, ` ▀████▀ `, `╵╷╵╷|╵╿╵╿`, `│ │ |║ ║ `},
		{` ▄████▄ `, `▐ .  . ▌`, `▐  ??  ▌`, ` ▀████▀ `, `│╷│╷|║╿║╿`, `╵ ╵ |╵ ╵ `},
		{` ▄████▄ `, `▐ .  o ▌`, `▐  ??  ▌`, ` ▀████▀ `, `╵╷╵╷|╵╿╵╿`, `│ │ |║ ║ `},
	},
}

// robotFrames[state][frame][line] — each line is exactly 8 visual columns.
var robotFrames = [3][3][6]string{
	{
		{`████████`, `█      █`, `█ ▀  ▄ █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ ─  ─ █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ ▄  ▀ █`, `█      █`, `██    ██`, ` ██████ `},
	},
	{
		{`████████`, `█      █`, `█ >  < █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ X  X █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ *  * █`, `█      █`, `██    ██`, ` ██████ `},
	},
	{
		{`████████`, `█      █`, `█ o  . █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ .  . █`, `█      █`, `██    ██`, ` ██████ `},
		{`████████`, `█      █`, `█ .  o █`, `█      █`, `██    ██`, ` ██████ `},
	},
}

// renderMascotLine colours one mascot line. Lines containing "|" are split:
// left half → styleMascotTentBlue, right half → styleMascotTentRed.
func renderMascotLine(line string, bodyStyle lipgloss.Style) string {
	if idx := strings.Index(line, "|"); idx >= 0 {
		return styleMascotTentBlue.Render(line[:idx]) + styleMascotTentRed.Render(line[idx+1:])
	}
	return bodyStyle.Render(line)
}
