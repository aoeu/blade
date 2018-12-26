package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	outputDirForGeneratedSourceFiles = "generated_java_sources"
	outputDirForBytecode             = "java_virtual_machine_bytecode"
	outputDexFilepath                = "classes.dex"
	filepathOfAPK                    = "app.apk"
	filepathOfUnalignedAPK           = "app.apk.unaligned"
)

func main() {
	args := struct {
		androidHome             string
		androidManifestFilepath string
		xmlResourcesFilepath    string
		javaSourcesFilepath     string
		outputDir               string
	}{}
	flag.StringVar(&args.androidHome, "sdk", "", "The location of the Android SDK to use in lieu of the environment variable $ANDROID_HOME (default)")
	flag.StringVar(&args.androidManifestFilepath, "manifest", "AndroidManifest.xml", "The location of the AndroidManifest.xml of the app to build in lieu of the current directory (default)")
	flag.StringVar(&args.xmlResourcesFilepath, "xml", "xml", "The parent-folder location of XML resources files (commonly 'res') to use in lieu of a folder named 'xml' within the current directory (default)")
	flag.StringVar(&args.javaSourcesFilepath, "java", "java", "The parent-folder location Java source files for the app to use in lieu of a folder named 'java' within the current directory (default)")
	flag.StringVar(&args.outputDir, "out", "", "The directory to output temporary built artifacts and final APK file, in lieu of the current directory")
	flag.Parse()
	if args.androidHome == "" {
		var envExists bool
		args.androidHome, envExists = os.LookupEnv("ANDROID_HOME")
		switch {
		case !envExists:
			fmt.Fprintf(os.Stderr, "ANDROID_HOME must be set as an environment variable or the SDK location must be provided manually as a flag\n")
			flag.Usage()
			os.Exit(1)
		case args.androidHome == "":
			fmt.Fprintf(os.Stderr, "ANDROID_HOME is set as an empty enviroment variable and must be non-empty, or the SDK location must be provided manually as a flag\n")
			flag.Usage()
			os.Exit(1)
		}
	}
	p, err := filepath.Abs(args.androidManifestFilepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could find AndroidManifest.xml at filepath '%v' due to error: '%v'\n", args.androidManifestFilepath, err)
		os.Exit(1)
	} else {
		args.androidManifestFilepath = p
	}

	p, err = filepath.Abs(args.outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not locate output directory at filepath '%v' due to error: %v\n", args.outputDir, err)
	} else {
		args.outputDir = p
	}

	// sdkmanager --install 'build-tools;28.0.3' 'platforms;android-28'

	t, err := newToolchain(args.androidHome)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not ascertain toolchain due to error: %v\n", err)
		os.Exit(1)
	}
	tmpDirs := []string{outputDirForGeneratedSourceFiles, outputDirForBytecode}
	if err := makeOutputDirs(tmpDirs...); err != nil {
		fmt.Fprintf(os.Stderr, "Could not create output directories due to error: %v\n", err)
		os.Exit(1)
	}
	if err = t.generateJavaFileForAndroidResources(args.outputDir+"/"+outputDirForGeneratedSourceFiles, args.androidManifestFilepath, args.xmlResourcesFilepath); err != nil {
		fmt.Fprintf(os.Stderr, "Could not create Java file from Android XML resources files due to error: %v\n", err)
		os.Exit(1)
	}

	err = t.compileJavaSourceFilesToJavaVirtualMachineBytecode(args.javaSourcesFilepath, outputDirForGeneratedSourceFiles, outputDirForBytecode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not compile java source files to bytecode due to error: %v\n", err)
		os.Exit(1)
	}

	err = t.translateJavaVirtualMachineMBytecodeToAndroidRuntimeBytecode(outputDexFilepath, outputDirForBytecode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not translate bytecode with dexer due to error: %v\n", err)
		os.Exit(1)
	}

	err = t.createUnalignedAndroidApplicationPackage(args.androidManifestFilepath, args.xmlResourcesFilepath, filepathOfUnalignedAPK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create unaligned APK file due to error: %v\n", err)
		os.Exit(1)
	}

	err = t.addAndroidRuntimeBytecodeToAndroidApplicationPackage(filepathOfUnalignedAPK, outputDexFilepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not add android runtime bytecode to APK due to error: %v\n", err)
		os.Exit(1)
	}

	err = t.signAndroidApplicationPackageWithDebugKey(filepathOfUnalignedAPK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not sign APK due to error: %v\n", err)
		os.Exit(1)
	}

	err = t.alignUncompressedDataInZipFileToFourByteBoundariesForFasterMemoryMappingAtRuntime(filepathOfUnalignedAPK, filepathOfAPK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could align bytes of APK file due to error: %v\n", err)
		os.Exit(1)
	}

	remove(append([]string{outputDexFilepath, filepathOfUnalignedAPK}, tmpDirs...)...)
}

func (t toolchain) alignUncompressedDataInZipFileToFourByteBoundariesForFasterMemoryMappingAtRuntime(filepathOfUnalignedAPK, filepathOfAPK string) error {
	return t.run(fmt.Sprintf("%v -f 4 %v %v", t.buildTools+"/zipalign", filepathOfUnalignedAPK, filepathOfAPK))
}

func (t toolchain) signAndroidApplicationPackageWithDebugKey(filepathOfUnalignedAPK string) error {
	// keytool -genkey -v -keystore debug.keystore -alias androiddebugkey -keyalg RSA -keysize 2048 -validity 10000 && mv debug.keystore $HOME/.android/
	return t.run(fmt.Sprintf("jarsigner -keystore %v/.android/debug.keystore -storepass android %v androiddebugkey", os.Getenv("HOME"), filepathOfUnalignedAPK))
}

func (t toolchain) addAndroidRuntimeBytecodeToAndroidApplicationPackage(filepathOfUnalignedAPK, outputDexFilepath string) error {
	return t.run(fmt.Sprintf("%v add %v %v", t.aaptBin, filepathOfUnalignedAPK, outputDexFilepath))
}

func (t toolchain) createUnalignedAndroidApplicationPackage(androidManifestFilepath, xmlResourcesFilepath, filepathOfUnalignedAPK string) error {
	return t.run(fmt.Sprintf("%v package -f -M %v -S %v -I %v -F %v", t.aaptBin, androidManifestFilepath, xmlResourcesFilepath, t.androidLib, filepathOfUnalignedAPK))

}
func (t toolchain) translateJavaVirtualMachineMBytecodeToAndroidRuntimeBytecode(outputDexFilepath, outputDirForBytecode string) error {
	return t.run(fmt.Sprintf("%v --dex --output %v %v", t.dxBin, outputDexFilepath, outputDirForBytecode))
}

func (t toolchain) compileJavaSourceFilesToJavaVirtualMachineBytecode(javaSourcesFilepath, outputDirForGeneratedSourceFiles, outputDirForBytecode string) error {
	j, err := findJavaSourceFiles(javaSourcesFilepath)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not find java source files to compile due to error: %v", err))
	}
	jj, err := findJavaSourceFiles(outputDirForGeneratedSourceFiles)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not find java source files to compile due to error: %v", err))
	}
	javaFiles := strings.Join(append(j, jj...), " ")
	return t.run(fmt.Sprintf("javac -classpath %v -sourcepath %v -d %v -target 1.7 -source 1.7 %v", t.androidLib, javaSourcesFilepath+":"+outputDirForGeneratedSourceFiles, outputDirForBytecode, javaFiles))
}

var javaFilename = regexp.MustCompile(`.*\.java$`)

func findJavaSourceFiles(rootDir string) ([]string, error) {
	fmt.Println(rootDir)
	paths := make([]string, 0)
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		switch {
		case err != nil:
			return err
		case info.IsDir():
		case javaFilename.MatchString(info.Name()):
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		err = errors.New(fmt.Sprintf("Received error when finding Java source files under '%v' : %v\n", rootDir, err))
	}
	return paths, err
}

func (t toolchain) generateJavaFileForAndroidResources(outputDirForGeneratedSourceFiles, manifestFilepath, resourcesFilepath string) error {
	// aapt package
	//
	//	Package the android resources.  It will read assets and resources that are
	//	supplied with the -M -A -S or raw-files-dir arguments.  The -J -P -F and -R
	//	options control which files are output.
	//
	//	-f  force overwrite of existing files
	//	-m  make package directories under location specified by -J
	//	-J  specify where to output R.java resource constant definitions
	//	-M  specify full path to AndroidManifest.xml to include in zip
	//	-S  directory in which to find resources.  Multiple directories will be scanned
	//		and the first match found (left to right) will take precedence.
	//	-I	add an existing package to base include set
	//
	// aapt package -f -m -J "$outputDirForGeneratedSourceFiles" -M "$manifestFilepath" -S "$resourcesFilepath" -I "$androidLib"
	return t.run(fmt.Sprintf("%v package -f -m -J %v -M %v -S %v -I %v", t.aaptBin, outputDirForGeneratedSourceFiles, manifestFilepath, resourcesFilepath, t.androidLib))

}

func (t toolchain) run(command string) error {
	s := strings.Split(spaces.ReplaceAllString(command, " "), " ")
	cmd := exec.Command(s[0], s[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.New(fmt.Sprintf("Error when running command %v : %v\n", command, err))
	}
	return nil
}

var spaces = regexp.MustCompile(`\s+`)

func remove(paths ...string) error {
	for _, s := range paths {
		f, err := os.Stat(s)
		if err != nil {
			return errors.New(fmt.Sprintf("Could not stat file at '%v' due to error: %v\n", s, err))
		}
		if f.IsDir() {
			if err := os.RemoveAll(s); err != nil {
				return errors.New(fmt.Sprintf("Could not remove directory at '%v' due to error: %v\n", s, err))
			}
		} else if err := os.Remove(s); err != nil {
			return err
		}
	}
	return nil
}

func makeOutputDirs(paths ...string) error {
	for _, s := range paths {
		if err := os.Mkdir(s, 0774); err != nil && !strings.Contains(err.Error(), "file exists") {
			return err
		}
	}
	return nil
}

type toolchain struct {
	sdk        string
	buildTools string
	platform   string
	androidLib string
	aaptBin    string
	dxBin      string
}

func newToolchain(SDKPath string) (toolchain, error) {
	t := toolchain{}
	var err error
	t.sdk, err = filepath.Abs(SDKPath)
	if err != nil {
		return t, errors.New(fmt.Sprintf("No valid directory has been found as $ANDROID_HOME due to error: %v", err))
	}

	p := t.sdk + "/build-tools"
	_, err = filepath.Abs(p)
	if err != nil {
		return t, errors.New(fmt.Sprintf("No build-tools directory found under '%v' due to error: %v", p, err))
	}
	d, err := os.Open(p)
	if err != nil {
		return t, errors.New(fmt.Sprintf("Could not find build-tools found under '%v' due to error: %v", p, err))
	}
	ff, err := d.Readdir(0)
	if err != nil {
		return t, errors.New(fmt.Sprintf("Could not find platforms under '%v' due to error: %v", p, err))
	}
	if len(ff) > 1 {
		return t, errors.New(fmt.Sprintf("No platforms found under '%v'", p))
	}
	indexOfMostRecentBuildToolsVersion := len(ff) - 1
	t.buildTools, err = filepath.Abs(p + "/" + ff[indexOfMostRecentBuildToolsVersion].Name())
	if err != nil {
		return t, errors.New(fmt.Sprintf("Received error when selecting most modern build-tools version: '%v'", err))
	}

	p = t.sdk + "/platforms"
	_, err = filepath.Abs(p)
	if err != nil {
		return t, errors.New(fmt.Sprintf("No valid platform found under '%v' due to error: %v", p, err))
	}

	d, err = os.Open(p)
	if err != nil {
		return t, errors.New(fmt.Sprintf("Could not find platforms under '%v' due to error: %v", p, err))
	}

	ff, err = d.Readdir(0)
	if err != nil {
		return t, errors.New(fmt.Sprintf("Could not find platforms under '%v' due to error: %v", p, err))
	}
	if len(ff) > 1 {
		return t, errors.New(fmt.Sprintf("No platforms found under '%v'", p))
	}

	indexOfMostRecentPlatformVersion := len(ff) - 1
	t.platform, err = filepath.Abs(p + "/" + ff[indexOfMostRecentPlatformVersion].Name())
	if err != nil {
		return t, errors.New(fmt.Sprintf("Received error when selecting most modern platform: '%v'", err))
	}

	p = t.platform + "/android.jar"
	t.androidLib, err = filepath.Abs(p)
	if err != nil {
		return t, errors.New(fmt.Sprintf("Could not find android.jar library at path '%v' due to error: '%v'", err))
	}

	p = t.buildTools + "/aapt"
	t.aaptBin, err = filepath.Abs(p)
	if err != nil {
		return t, errors.New(fmt.Sprintf("Could not find aapt binary at path '%v' due to error: '%v'", err))
	}

	p = t.buildTools + "/dx"
	t.dxBin, err = filepath.Abs(p)
	if err != nil {
		return t, errors.New(fmt.Sprintf("Could not find dx binary at path '%v' due to error: '%v'", err))
	}
	return t, nil
}