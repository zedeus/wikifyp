plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.compose.compiler)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.roborazzi)
}

android {
    namespace = "com.wikifyp"
    compileSdk =
        libs.versions.compileSdk
            .get()
            .toInt()
    defaultConfig {
        applicationId = "com.wikifyp"
        minSdk =
            libs.versions.minSdk
                .get()
                .toInt()
        targetSdk =
            libs.versions.compileSdk
                .get()
                .toInt()
        versionCode = 1
        versionName = "1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        // Feed server root. Explicit -PserverUrl always wins; an empty
        // value serves the bundled fixtures.
        buildConfigField("String", "SERVER_URL", "\"${project.findProperty("serverUrl") ?: ""}\"")
    }

    buildTypes {
        debug {
            // A plain debug install talks to the dev server on the
            // emulator's host (10.0.2.2 is the emulator's alias for the
            // host's loopback; `./dev server` and no adb reverse needed).
            // -PserverUrl still overrides, and instrumented tests fall
            // back to fixtures at runtime regardless (MainActivity).
            buildConfigField(
                "String",
                "SERVER_URL",
                "\"${project.findProperty("serverUrl") ?: "http://10.0.2.2:8080"}\"",
            )
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    testOptions {
        unitTests {
            isIncludeAndroidResources = true
            all {
                // Robolectric reaches into FileDescriptor internals; closed
                // off by default since JDK 17 strong encapsulation.
                it.jvmArgs("--add-exports", "java.base/jdk.internal.access=ALL-UNNAMED")
            }
        }
    }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
}

// Debug unit tests are THE JVM suite; `check` would otherwise run every
// Robolectric/Roborazzi test once per build type for identical coverage.
// Stated as an include-rule so a future build type doesn't silently
// reintroduce the duplicate runs.
androidComponents {
    beforeVariants { variant ->
        variant.hostTests[com.android.build.api.variant.HostTestBuilder.UNIT_TEST_TYPE]
            ?.enable = variant.buildType == "debug"
    }
}

kotlin {
    jvmToolchain(
        libs.versions.jdk
            .get()
            .toInt(),
    )
}

roborazzi {
    outputDir.set(file("src/test/screenshots"))
}

dependencies {
    implementation(project(":reader"))

    val composeBom = platform(libs.androidx.compose.bom)
    implementation(composeBom)
    testImplementation(composeBom)
    androidTestImplementation(composeBom)

    // Core Android dependencies
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.kotlinx.coroutines.android)

    // Compose
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.material.icons.core)
    // Tooling
    debugImplementation(libs.androidx.compose.ui.tooling)

    // Local tests: JUnit, Robolectric, Compose on the JVM, Roborazzi screenshots
    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.robolectric)
    testImplementation(libs.roborazzi)
    testImplementation(libs.roborazzi.compose)
    testImplementation(libs.androidx.compose.ui.test.junit4)
    testImplementation(libs.androidx.compose.ui.test.manifest)

    // Instrumented tests
    androidTestImplementation(libs.androidx.compose.ui.test.junit4)
    debugImplementation(libs.androidx.compose.ui.test.manifest)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.androidx.test.runner)
}
