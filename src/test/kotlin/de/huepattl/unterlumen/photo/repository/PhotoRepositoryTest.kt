package de.huepattl.unterlumen.photo.repository

import org.junit.jupiter.api.Assertions.*
import org.junit.jupiter.api.Test
import java.io.File
import java.util.UUID

class PhotoRepositoryTest {

    private val testRepoLocation = "/tmp/unterlumen"

    @Test
    fun `store bird`() {
        val repo = PhotoRepository(
            repositoryLocation = testRepoLocation,
        )

        val photo = repo.store(
            id = UUID.randomUUID(),
            source = getTestResourcePath("images/fujifilm_x-t50_bird.jpeg")
        )

        assertTrue(File(photo.handle).exists())
    }

    private fun getTestResourcePath(filename: String): String {
        val resource = javaClass.classLoader.getResource(filename)
        assertNotNull(resource, "file not found in resources: $filename")

        val file = File(resource!!.toURI())
        return file.absolutePath
    }

}