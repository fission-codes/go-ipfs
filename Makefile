all:
	@gmake $@
.PHONY: all

.DEFAULT:
	@gmake $@

build-carmirror:
	@gmake setup-kubo-build
