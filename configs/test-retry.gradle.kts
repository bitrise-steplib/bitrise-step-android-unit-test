initscript {
    repositories {
        gradlePluginPortal()
    }
    dependencies {
        classpath("org.gradle.test-retry:org.gradle.test-retry.gradle.plugin:1.6.2")
    }
}

allprojects {
    apply<org.gradle.testretry.TestRetryPlugin>()
    tasks.withType<Test>().configureEach {
        extensions.findByName("retry")?.let { retryExt ->
            try {
                retryExt.javaClass.getMethod("setProperty", String::class.java, Object::class.java)
                    .invoke(retryExt, "maxRetries", 3)
            } catch (e: Exception) {
                // Ignore if method does not exist
            }
        }
    }
}