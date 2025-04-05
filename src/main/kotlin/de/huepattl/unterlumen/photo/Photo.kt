package de.huepattl.unterlumen.photo

import java.util.*

data class Photo(
    val id: UUID,
    val handle: String? = null,
    val metadata: Metadata? = null,
    //val data?: ByteArray
)
