package de.huepattl.unterlumen.photo.repository

import de.huepattl.unterlumen.photo.Photo
import io.quarkus.logging.Log
import jakarta.enterprise.context.ApplicationScoped
import org.eclipse.microprofile.config.inject.ConfigProperty
import java.io.File
import java.util.*


@ApplicationScoped
class PhotoRepository(
    @ConfigProperty(name = "unterlumen.repository.location")
    private val repositoryLocation: String,
) {

    init {
        ensureLocation(repositoryLocation)
    }

    fun store(id: UUID, source: String): Photo {
        if (fileExists(source)) {
            ensureLocation(repositoryLocation)
            val dest = "$repositoryLocation/${id}"
            Log.info("storing photo '$source' to '$dest'")
            copy(source, dest)

            return Photo(id = id, handle = dest)
        } else {
            throw IllegalArgumentException("photo $source does not exist")
        }
    }

    fun retrieve(id: UUID): Photo? {
        return Photo(id = id, handle = "$repositoryLocation/${id}")
    }

    private fun ensureLocation(folder: String) {
        val dir = File(folder)
        if (!dir.exists()) {
            dir.mkdirs()
        }
    }

    private fun fileExists(path: String): Boolean {
        return File(path).exists()
    }

    private fun copy(from: String, to: String) {
        val src = File(from)
        val dest = File(to)
        src.copyTo(dest, overwrite = true)
    }

}