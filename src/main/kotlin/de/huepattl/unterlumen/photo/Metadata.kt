package de.huepattl.unterlumen.photo

import de.huepattl.unterlumen.photo.types.Dimensions
import de.huepattl.unterlumen.photo.types.Exposure
import de.huepattl.unterlumen.photo.types.Lens
import de.huepattl.unterlumen.photo.types.Orientation
import de.huepattl.unterlumen.photo.types.Quality

data class Metadata(
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
)