# Breez SDK - Spark

## **What is the Breez SDK?**

The Breez SDK provides developers with an end-to-end solution for integrating self-custodial Lightning into their apps and services. It eliminates the need for third parties, simplifies the complexities of Bitcoin and Lightning, and enables seamless onboarding for billions of users to the future of value transfer.

## **What is the Breez SDK - Spark?**

It’s a nodeless integration that offers a self-custodial, end-to-end solution for integrating Lightning payments, utilizing Spark with on-chain interoperability and third-party fiat on-ramps.

## Installation

To install the package:

```sh
$ go get github.com/breez/breez-sdk-spark-go
```

### Supported platforms

This package embeds the Breez SDK - Spark runtime compiled as shared library objects, and uses [`cgo`](https://golang.org/cmd/cgo/) to consume it. A set of precompiled shared library objects are provided. Thus this package works (and is tested) on the following platforms:

<table>
  <thead>
    <tr>
      <th>Platform</th>
      <th>Architecture</th>
      <th>Triple</th>
      <th>Status</th>
      <th>Bundling</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td rowspan="2">Android</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-linux-android</code></td>
      <td>✅</td>
      <td>See <a href="#android">Android</a></td>
    </tr>
    <tr>
      <td><code>aarch64</code></td>
      <td><code>aarch64-linux-android</code></td>
      <td>✅</td>
      <td>See <a href="#android">Android</a></td>
    </tr>
    <tr>
      <td rowspan="2">Darwin (macOS)</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-apple-darwin</code></td>
      <td>✅</td>
      <td>See <a href="#darwin-macos">Darwin (macOS)</a></td>
    </tr>
    <tr>
      <td><code>aarch64</code></td>
      <td><code>aarch64-apple-darwin</code></td>
      <td>✅</td>
      <td>See <a href="darwin-macos">Darwin (macOS)</a></td>
    </tr>
    <tr>
      <td rowspan="2">iOS</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-apple-ios</code></td>
      <td>✅</td>
      <td>See <a href="#ios">iOS</a></td>
    </tr>
    <tr>
      <td><code>aarch64</code></td>
      <td><code>aarch64-apple-ios</code></td>
      <td>✅</td>
      <td>See <a href="#ios">iOS</a></td>
    </tr>
    <tr>
      <td rowspan="2">Linux</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-unknown-linux-gnu</code></td>
      <td>✅</td>
      <td></td>
    </tr>
    <tr>
      <td><code>aarch64</code></td>
      <td><code>aarch64-unknown-linux-gnu</code></td>
      <td>✅</td>
      <td></td>
    </tr>
    <tr>
      <td>Windows</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-pc-windows-msvc</code></td>
      <td>✅</td>
      <td>See <a href="#windows">Windows</a></td>
    </tr>
  </tbody>
</table>

## Usage

Head over to the Breez SDK - Spark [documentation](https://sdk-doc-spark.breez.technology/) to start implementing Lightning in your app.

```go
package main

import (
    "github.com/breez/breez-sdk-spark-go/breez_sdk_spark"
)

func main() {
    mnemonic := "<mnemonic words>"
    var seed breez_sdk_spark.Seed = breez_sdk_spark.SeedMnemonic{
        Mnemonic:   mnemonic,
        Passphrase: nil,
    }

    apiKey := "<breez api key>"
    config := breez_sdk_spark.DefaultConfig(breez_sdk_spark.NetworkMainnet)
    config.ApiKey = &apiKey

    connectRequest := breez_sdk_spark.ConnectRequest{
        Config:     config,
        Seed:       seed,
        StorageDir: "./.data",
    }

    sdk, err := breez_sdk_spark.Connect(connectRequest)
}
```

## Bundling

For [Android](#android) and [Windows](#windows), the provided binding libraries must be copied to a location that can be found at runtime.  

For [iOS](#ios), the native binary framework must be installed in addition using [Swift Package Manager](#swift-package-manager) or [CocoaPods](#cocoapods).

### Android

Copy the binding libraries into the jniLibs directory of your app
```bash
cp vendor/github.com/breez/breez-sdk-spark-go/breez_sdk_spark/lib/android-aarch64/*.so android/app/src/main/jniLibs/arm64-v8a/
cp vendor/github.com/breez/breez-sdk-spark-go/breez_sdk_spark/lib/android-amd64/*.so android/app/src/main/jniLibs/x86_64/
```
So they are in the following structure
```
└── android
    ├── app
        └── src
            └── main
                └── jniLibs
                    ├── arm64-v8a
                        ├── libbreez_sdk_spark_bindings.so
                        └── libc++_shared.so
                    └── x86_64
                        ├── libbreez_sdk_spark_bindings.so
                        └── libc++_shared.so
                └── AndroidManifest.xml
        └── build.gradle
    └── build.gradle
```

### Darwin (macOS)

For development, `go run` and `go build` work out of the box since the bundled `.dylib` is referenced via `rpath` pointing into the Go module cache.

For deployment, create a universal dylib and place it in your app bundle's Frameworks directory:

```bash
lipo -create \
  vendor/github.com/breez/breez-sdk-spark-go/breez_sdk_spark/lib/darwin-aarch64/libbreez_sdk_spark_bindings.dylib \
  vendor/github.com/breez/breez-sdk-spark-go/breez_sdk_spark/lib/darwin-amd64/libbreez_sdk_spark_bindings.dylib \
  -output YourMacOSApp/Contents/Frameworks/libbreez_sdk_spark_bindings.dylib
```

### iOS

When targeting iOS, you must also install the native binary framework. This is the same framework used by the Swift Breez SDK package and can be installed via [Swift Package Manager](#swift-package-manager) or [CocoaPods](#cocoapods).

**Note:** The Go and Swift packages (installed via SPM or CocoaPods) **MUST** have the same version. A version mismatch between the two will cause linking or runtime errors.


#### Swift Package Manager

##### Installation via Xcode

Via `File > Add Packages...`, add

```
https://github.com/breez/breez-sdk-spark-swift.git
```

as a package dependency in Xcode.

##### Installation via Swift Package Manifest

Add the following to the dependencies array of your `Package.swift`:

``` swift
.package(url: "https://github.com/breez/breez-sdk-spark-swift.git"),
```

#### CocoaPods

Add the Breez SDK to your `Podfile` like so and run `pod install`:

``` ruby
target '<YourApp>' do
  use_frameworks!
  pod 'breez_sdk_sparkFFI'
end
```

### Windows

Copy the binding library to the same directory as the executable file or include the library into the windows install packager.
```bash
cp vendor/github.com/breez/breez-sdk-spark-go/breez_sdk_spark/lib/windows-amd64/*.dll build/windows/
```

## Pricing

The Breez SDK is **free** for developers. 

## Support

Have a question for the team? Join us on [Telegram](https://t.me/breezsdk) or email us at <contact@breez.technology>.

## Information for Maintainers and Contributors

This repository is used to publish a Go package providing Go bindings to the Breez SDK - Spark's [underlying Rust implementation](https://github.com/breez/spark-sdk). The Go bindings are generated using [UniFFi Bindgen Go](https://github.com/NordSecurity/uniffi-bindgen-go).

Any changes to Breez SDK - Spark, the Go bindings, and the configuration of this Go package must be made via the [spark-sdk](https://github.com/breez/spark-sdk) repository.
