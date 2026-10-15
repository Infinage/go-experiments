.PHONY: dev watch-css watch-go

dev:
	@$(MAKE) -j2 watch-css watch-go

watch-css:
	tailwindcss -i assets/input.css -o assets/index.css --watch

watch-go:
	 air
