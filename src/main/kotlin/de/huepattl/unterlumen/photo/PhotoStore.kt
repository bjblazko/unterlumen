package de.huepattl.unterlumen.photo

import de.huepattl.unterlumen.photo.repository.MetadataRepository
import de.huepattl.unterlumen.photo.repository.PhotoRepository
import de.huepattl.unterlumen.photo.types.Dimensions
import jakarta.enterprise.context.ApplicationScoped
import jakarta.inject.Inject
import jakarta.transaction.Transactional
import org.eclipse.microprofile.config.inject.ConfigProperty
import java.io.File
import java.nio.file.Files
import java.util.UUID
import javax.swing.Spring.height
import kotlin.io.path.Path

@ApplicationScoped
@Transactional
class PhotoStore @Inject constructor(
    private val metadataRepository: MetadataRepository,
    private val photoRepository: PhotoRepository,
    private val exifMetadataRetriever: ExifMetadataRetriever,
    private val imageScaler: ImageScaler,
    @ConfigProperty(name = "unterlumen.cache.location")
    private val cacheLocation: String,
) {

    fun digest(filename: String): Photo {
        checkFile(filename)

        val metadata = extractMetadata(filename)
        metadataRepository.persist(metadata)

        val photo = photoRepository.store(id = metadata.id, source = filename)

        return photo.copy(metadata = metadata)
    }

    fun retrieve(id: UUID): Photo? {
        val metadata = metadataRepository.findById(id.toString())?.toDomainObject()
        val photo = photoRepository.retrieve(id)
        return if (metadata != null && photo != null) {
            photo.copy(metadata = metadata)
        } else {
            null
        }
    }

    fun retrieveScaled(id: UUID, width: Int): Photo? {
        var cachedPhotoPath = "$cacheLocation/${id}_${width}"
        if (!Files.exists(Path(cachedPhotoPath))) {
            println("Image not found: $cachedPhotoPath")
            val original = retrieve(id)
            ensureLocation(cacheLocation)
            val height = imageScaler.scaleToWidth(
                source = original?.handle!!,
                width = width,
                dest = cachedPhotoPath
            )
            return original
                .copy(handle = cachedPhotoPath)
                .copy(
                    metadata = original.metadata?.copy(
                        dimensions =
                            Dimensions.of(height = height, width = width)
                    )
                )
        } else {
            val metadata = metadataRepository.findById(id.toString())?.toDomainObject()
            return Photo(
                id = id,
                metadata = metadata?.copy(dimensions = Dimensions.of(height = 0, width = width)), // fixme height
                handle = cachedPhotoPath
            )
        }
    }

    internal fun checkFile(filename: String) {
        val file = File(filename)
        if (!file.exists()) {
            throw IllegalArgumentException("file '$filename' does not exist")
        }
    }

    internal fun extractMetadata(filename: String): Metadata = exifMetadataRetriever.fromFile(filename)

    private fun ensureLocation(folder: String) {
        val dir = File(folder)
        if (!dir.exists()) {
            dir.mkdirs()
        }
    }

}