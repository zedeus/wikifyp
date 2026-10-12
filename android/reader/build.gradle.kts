plugins {
    alias(libs.plugins.android.library)
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
    compileOptions {
        sourceCompatibility = JavaVersion.toVersion(libs.versions.jdk.get())
        targetCompatibility = JavaVersion.toVersion(libs.versions.jdk.get())
    }
}

kotlin {
    jvmToolchain(
        libs.versions.jdk
            .get()
            .toInt(),
    )
}

// The reader must stay buildable and testable on its own: no dependency on
// any other module of this project (DIRECTION.md). Wired into `check` so
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

// Hooked into both `check` (CI) and `test` (the local verification command),
// so an added dependency fails before the commit, not after the push.
tasks.named("check") {
    dependsOn(checkReaderIsolation)
}

// AGP registers `test` after evaluation, so match lazily instead of named().
tasks.matching { it.name == "test" }.configureEach {
    dependsOn(checkReaderIsolation)
}
