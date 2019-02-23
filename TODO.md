# TODO

* Detect whether debug signing key is present on a system, install it.
	`$ keytool -genkey -v -keystore debug.keystore -storepass android -alias androiddebugkey -keypass android -keyalg RSA -keysize 2048 -validity 10000`

* Validate that XML arguments Detect if AndroidManifest.xml file exists up-front before running any build commands

