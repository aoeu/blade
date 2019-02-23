bustler: bustler-on-unix

projectLocationOnWindows="/mnt/c/Users/aoeu/Documents/Github"
sdkLocationOnWSL="/home/aoeu/dev/android-sdk"

projectLocationOnUnix="/home/aoeu/bustler"
sdkLocationOnUnix="/home/aoeu/android"

bustler-on-winows-with-WSL:
	go run build.go \
		-sdk "$$sdkLocationOnWSL" \
		-manifest "$$projectLocationOnWindows/AndroidManifest.xml" \
		-xml "$$projectLocationOnWindows/xml" \
		-java "$$projectLocationOnWindows/java"

bustler-on-unix:
	go run build.go \
		-sdk "$$sdkLocationOnUnix" \
		-manifest "$$projectLocationOnUnix/AndroidManifest.xml" \
		-xml "$$projectLocationOnUnix/xml" \
		-java "$$projectLocationOnUnix/java"


