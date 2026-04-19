package tui

func splitWidth(totalWidth int) (left, right int) {
	left = min(totalWidth/3, 50)
	right = max(totalWidth-left, 0)
	return
}
