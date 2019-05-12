# TODO

* Install it a debug singing key automatically?
	`$ keytool -genkey -v -keystore debug.keystore -storepass android -alias androiddebugkey -keypass android -keyalg RSA -keysize 2048 -validity 10000`

* Validate that XML arguments Detect if AndroidManifest.xml file exists up-front before running any build commands

* Validate the contents of AndroidManifest.xml against any kind of XML parser. Surprisingly, putting garbage text into the file at random places outside of tags (i.e. accidentally putting an attribute after the tag has closed) does not cause any visible error from Android (maybe there is one in logcat), and it happily installs the apps even with bogus XML in the manifest file. 

