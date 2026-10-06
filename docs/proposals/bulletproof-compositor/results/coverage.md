files merged: 40; advertised globals seen: 47; interfaces reachable (globals plus objects they create): 110
requests in those interfaces: 302; sent at least once: 301 (100%); sent at least 20 times: 277 (92%); never sent: 1

advertised globals (name version): ext_background_effect_manager_v1 v1, ext_data_control_manager_v1 v1, ext_foreign_toplevel_image_capture_source_manager_v1 v1, ext_foreign_toplevel_list_v1 v1, ext_idle_notifier_v1 v2, ext_image_copy_capture_manager_v1 v1, ext_output_image_capture_source_manager_v1 v1, ext_session_lock_manager_v1 v1, ext_workspace_manager_v1 v1, wl_compositor v6, wl_data_device_manager v3, wl_output v4, wl_seat v9, wl_shm v2, wl_subcompositor v1, wp_content_type_manager_v1 v1, wp_cursor_shape_manager_v1 v2, wp_fractional_scale_manager_v1 v1, wp_presentation v2, wp_security_context_manager_v1 v1, wp_single_pixel_buffer_manager_v1 v1, wp_viewporter v1, xdg_activation_v1 v1, xdg_wm_base v7, xdg_wm_dialog_v1 v1, zwlr_data_control_manager_v1 v2, zwlr_foreign_toplevel_manager_v1 v3, zwlr_gamma_control_manager_v1 v1, zwlr_layer_shell_v1 v5, zwlr_output_manager_v1 v4, zwlr_output_power_manager_v1 v1, zwlr_screencopy_manager_v1 v3, zwp_idle_inhibit_manager_v1 v1, zwp_input_method_manager_v2 v1, zwp_keyboard_shortcuts_inhibit_manager_v1 v1, zwp_linux_dmabuf_v1 v3, zwp_pointer_constraints_v1 v1, zwp_pointer_gestures_v1 v3, zwp_primary_selection_device_manager_v1 v1, zwp_relative_pointer_manager_v1 v1, zwp_tablet_manager_v2 v1, zwp_text_input_manager_v3 v1, zwp_virtual_keyboard_manager_v1 v1, zxdg_decoration_manager_v1 v1, zxdg_exporter_v2 v1, zxdg_importer_v2 v1, zxdg_output_manager_v1 v3

never sent:
  ext_image_copy_capture_frame_v1.damage_buffer

protocol errors that disconnected a fuzz client (count):
    5398 xdg_wm_base#3: an xdg_popup cannot be its own ancestor
    4487 xdg_surface#3: must ack the initial configure before attaching buffer
    4441 xdg_positioner#0: Invalid size for positioner.
    2980 xdg_positioner#0: Invalid size for positioner's anchor rectangle.
    2653 wl_display#0: Unknown id: 4294967295.
    2492 wl_surface#3: Passing non-zero x,y is protocol violation since versions 5
    1482 zwlr_layer_shell_v1#1: invalid layer: (InvalidLayer, "invalid layer: Unknown(4)")
    1442 zwlr_layer_shell_v1#1: invalid layer: (InvalidLayer, "invalid layer: Unknown(5)")
    1094 zwlr_layer_surface_v1#3: wrong keyboard interactivity value: Unknown(3)
     700 wl_subsurface#0: Provided surface is not a sibling or parent.
     630 zwlr_layer_surface_v1#1: width 0 requested without setting left and right anchors
     624 zwlr_layer_surface_v1#1: height 0 requested without setting top and bottom anchors
     531 wl_shm#1: invalid wl_shm_pool size
     506 wp_viewport#0: negative or zero values in width or height
     494 wp_viewport#0: negative or zero values in width or height or negative value
     477 wl_display#1: invalid method 3 (since 3 < 4), object zwp_linux_dmabuf_v1#5
     470 wl_display#1: invalid method 2 (since 3 < 4), object zwp_linux_dmabuf_v1#5
     294 zwlr_layer_surface_v1#2: invalid anchor Unknown(18)
     281 zwlr_layer_surface_v1#2: invalid anchor Unknown(16)
     272 zwlr_layer_surface_v1#2: invalid anchor Unknown(19)
     266 zwlr_layer_surface_v1#0: must ack the initial configure before attaching buffer
     263 zwlr_layer_surface_v1#2: invalid anchor Unknown(20)
     251 zwlr_layer_surface_v1#2: invalid anchor Unknown(17)
     219 wl_surface#0: Scale must be positive
     218 wl_display#0: Invalid object 1 in request ext_image_copy_capture_manager_v
     218 wl_display#0: Invalid new_id: 60.
     214 wl_display#0: Invalid new_id: 61.
     214 wl_display#0: Invalid object 50 in request ext_image_copy_capture_manager_
     213 wl_display#0: Invalid null object in request ext_image_copy_capture_manage
     213 wl_display#0: Invalid object 2 in request ext_image_copy_capture_manager_v
     204 wl_display#0: Invalid new_id: 16777216.
     197 wl_subcompositor#0: Surface already has a role.
     185 xdg_toplevel#1: invalid parent toplevel
     175 wl_display#0: Invalid new_id: 0.
     164 wl_display#0: Invalid object 50 in request ext_foreign_toplevel_image_capt
     162 wl_display#0: Invalid null object in request ext_foreign_toplevel_image_ca
     159 wl_display#0: Invalid object 2 in request ext_foreign_toplevel_image_captu
     156 wl_display#0: Invalid object 1 in request ext_foreign_toplevel_image_captu
     153 zxdg_exporter_v2#0: exported surface had an invalid role
     152 wl_display#0: Invalid object 1 in request xdg_wm_dialog_v1.get_xdg_dialog:
