package de.huepattl.unterlumen.photo

import de.huepattl.unterlumen.photo.types.*
import java.time.LocalDateTime
import java.util.*

data class Metadata(
    val id: UUID = UUID.randomUUID(),
    val title: String? = null,
    val filename: String? = null,
    val fileSize: FileSize? = null,
    val mimeType: String? = null,
    val description: String? = null,
    val tags: List<String> = listOf(),
    val dimensions: Dimensions? = null,
    val exposure: Exposure? = null,
    val quality: Quality? = null,
    val cameraBrand: String? = null,
    val cameraModel: String? = null,
    val software: String? = null,
    val artist: String? = null,
    val copyright: String? = null,
    val orientation: Orientation? = null,
    val lens: Lens? = null,
    val whiteBalanceMode: String? = null,
    val look: String? = null,
    val geoLocation: GeoLocation? = null,
    val createdAt: LocalDateTime? = null,
)