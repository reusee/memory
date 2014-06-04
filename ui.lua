Stage{
	id = "stage",
	bgcolor = "#000015",
	layout = VBox{},
	Actor{ y_expand = true }, -- padding
	Text{
		id = "text",
		color = "#0099CC",
		use_markup = true,
	},
	Text{
		id = "hint",
		color = "#EEEEEE",
	},
	Text{
		id = "history",
		color = "#666666",
		margin_top = 20,
	},
	Actor{ y_expand = true }, -- padding
}
