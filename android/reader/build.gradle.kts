plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "com.wikifyp.reader"
    compileSdk =
        libs.versions.compileSdk
            .get()
            .toInt()
    defaultConfig {
        minSdk =
            libs.versions.minSdk
                .get()
                .toInt()
    }
    // Shared fixtures (tokens.json, events/) on the test classpath so both
    // Kotlin and Go validate against the same files.
    sourceSets {
        getByName("test") {
            resources.directories.add("../../fixtures")
        }
    }
}

// No instrumented tests here yet; skip building and installing an empty test
// APK on every emulator run. Re-enable when the reader grows UI tests.
androidComponents {
    beforeVariants { variant ->
        variant.androidTest.enable = false
    }
}

kotlin {
    jvmToolchain(
        libs.versions.jdk
            .get()
            .toInt(),
    )
}

dependencies {
    implementation(libs.kotlinx.serialization.json)
    testImplementation(libs.junit)
}

// The reader must stay buildable and testable on its own: no dependency on
// any other module of this project. Wired into `check` so
// `./gradlew check`/CI fails the moment someone adds one.
val checkReaderIsolation by tasks.registering {
    val projectDeps =
        configurations
            .flatMap { configuration ->
                configuration.dependencies
                    .withType(ProjectDependency::class.java)
                    // Test classpaths depend on the module itself; only other modules count.
                    .filter { it.path != project.path }
                    .map { "${configuration.name} -> ${it.path}" }
            }
    doLast {
        check(projectDeps.isEmpty()) {
            ":reader must not depend on other modules, found: $projectDeps"
        }
    }
}

// Hooked into `test`, which both the local command and CI's `check` reach
// (check depends on test). AGP registers `test` after evaluation, so match
// lazily instead of named().
tasks.matching { it.name == "test" }.configureEach {
    dependsOn(checkReaderIsolation)
}
